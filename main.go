package main

import (
	"context"
	"fmt"
	"os"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FeeeLyX/tg-tui/internal/app"
	"github.com/FeeeLyX/tg-tui/internal/app/usecase"
	"github.com/FeeeLyX/tg-tui/internal/ports/inbound"
	"github.com/FeeeLyX/tg-tui/internal/ports/outbound"
	"github.com/FeeeLyX/tg-tui/internal/storage"
	service "github.com/FeeeLyX/tg-tui/internal/telegram"
	"github.com/FeeeLyX/tg-tui/internal/ui"
)

const defaultVersion = "v0.1.0"

var (
	version = defaultVersion
	commit  = "dev"
	date    = "unknown"
)

func main() {
	if shouldPrintVersion(os.Args[1:]) {
		fmt.Printf("tg-tui %s\n", version)
		fmt.Printf("commit: %s\n", commit)
		fmt.Printf("built:  %s\n", date)
		fmt.Printf("go:     %s\n", runtime.Version())
		return
	}

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "tg-tui: %v\n", err)
		os.Exit(1)
	}
}

func shouldPrintVersion(args []string) bool {
	for _, arg := range args {
		if arg == "-version" || arg == "--version" || arg == "version" {
			return true
		}
	}

	return false
}

func run() error {
	config, err := app.LoadConfig()
	if err != nil {
		return err
	}

	logger, err := app.NewLogger(config)
	if err != nil {
		return err
	}
	defer logger.Close()

	logger.Infof("startup begin")
	logger.Infof("credential source: %s", config.CredentialSource)
	altScreenEnabled := os.Getenv("TG_TUI_ALT_SCREEN") != "0"
	logger.Infof("alt screen enabled: %t", altScreenEnabled)
	logger.Infof("term env: TERM=%q COLORTERM=%q", os.Getenv("TERM"), os.Getenv("COLORTERM"))

	stdinInfo, stdinErr := os.Stdin.Stat()
	stdoutInfo, stdoutErr := os.Stdout.Stat()
	if stdinErr != nil || stdoutErr != nil {
		logger.Errorf("terminal stat failed: stdin=%v stdout=%v", stdinErr, stdoutErr)
		return fmt.Errorf("unable to inspect terminal: stdin=%v stdout=%v", stdinErr, stdoutErr)
	}
	logger.Infof("stdin mode: %s", stdinInfo.Mode().String())
	logger.Infof("stdout mode: %s", stdoutInfo.Mode().String())

	if (stdinInfo.Mode()&os.ModeCharDevice) == 0 || (stdoutInfo.Mode()&os.ModeCharDevice) == 0 {
		logger.Errorf("interactive terminal check failed")
		return fmt.Errorf("tg-tui requires an interactive terminal; run with `go run .` in a real shell")
	}
	logger.Infof("interactive terminal check passed")

	logger.Infof("opening cache: %s", config.CachePath)
	cache, err := storage.OpenCache(config.CachePath)
	if err != nil {
		logger.Errorf("open cache failed: %v", err)
	} else {
		defer cache.Close()
		logger.Infof("cache opened: %s", config.CachePath)
	}

	ctx := context.Background()

	var boot inbound.Bootstrapper = usecase.Bootstrapper{
		NewTelegramClient: func(cfg app.Config) outbound.TelegramClient {
			return service.NewClient(cfg)
		},
		Logger: logger,
	}
	state, tgClient := boot.Run(ctx, config, cache)
	if tgClient != nil {
		defer tgClient.Close()
	}
	if cache == nil && state.Error == nil {
		state.Status = "Cache unavailable, running without local cache"
	}

	programOptions := []tea.ProgramOption{}
	if altScreenEnabled {
		programOptions = append(programOptions, tea.WithAltScreen())
	}
	programOptions = append(programOptions, tea.WithMouseCellMotion())

	program := tea.NewProgram(ui.NewModel(state, tgClient), programOptions...)
	logger.Infof("bubbletea program run start")
	_, err = program.Run()
	if err != nil {
		logger.Errorf("bubbletea program run failed: %v", err)
	} else {
		logger.Infof("bubbletea program run exited cleanly")
	}
	return err
}
