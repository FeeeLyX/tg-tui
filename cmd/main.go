package main

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"tg-tui/internal/app"
	"tg-tui/internal/storage"
	service "tg-tui/internal/telegram"
	"tg-tui/internal/ui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "tg-tui: %v\n", err)
		os.Exit(1)
	}
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
	logger.Infof("alt screen enabled: %t", os.Getenv("TG_TUI_ALT_SCREEN") == "1")
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
		return fmt.Errorf("tg-tui requires an interactive terminal; run with `go run ./cmd` in a real shell")
	}
	logger.Infof("interactive terminal check passed")

	state := app.NewState()
	state.Status = "Session bootstrap pending"
	state.CredentialSummary = config.CredentialSummary()
	state.CredentialNotice = config.CredentialNotice()
	var tgClient app.TelegramClient

	logger.Infof("opening cache: %s", config.CachePath)
	cache, err := storage.OpenCache(config.CachePath)
	if err != nil {
		logger.Errorf("open cache failed: %v", err)
		state.Status = "Cache unavailable, running without local cache"
	} else {
		defer cache.Close()
		logger.Infof("cache opened: %s", config.CachePath)
	}

	ctx := context.Background()
	if cache != nil {
		chats, err := cache.LoadChats(ctx)
		if err == nil && len(chats) > 0 {
			state = ui.ApplyCachedChats(state, chats)
			if state.ActiveChatID != 0 {
				messages, loadErr := cache.LoadMessages(ctx, state.ActiveChatID)
				if loadErr == nil {
					state = ui.ApplyCachedMessages(state, state.ActiveChatID, messages)
				}
			}
			state.Status = "Loaded cached data"
		}
	}

	if err := config.ValidateCredentials(); err != nil {
		logger.Errorf("credential validation failed: %v", err)
		state.Error = err
		state.Status = "Telegram credentials required"
	} else {
		logger.Infof("credential validation passed")
		tgClient = service.NewClient(config)
		logger.Infof("telegram client start begin")
		startErr := tgClient.Start(ctx)
		if startErr != nil {
			logger.Errorf("telegram client start failed: %v", startErr)
			state.Error = fmt.Errorf("telegram startup failed: %w", startErr)
			state.Status = "Telegram startup failed"
			_ = tgClient.Close()
			tgClient = nil
		} else {
			defer tgClient.Close()
			logger.Infof("telegram client started")

			session, err := tgClient.Session(ctx)
			if err != nil {
				state.Error = err
				state.Status = "Telegram session bootstrap failed"
			} else {
				state.Session = session
			}

			authState, err := tgClient.AuthState(ctx)
			if err == nil {
				state.AuthState = authState
			}
			logger.Infof("telegram auth state step: %s", state.AuthState.Step)

			if state.Session.Authorized {
				state.Status = "Authorized. Chat sync is next."
			} else {
				state.Status = "Awaiting Telegram login"
			}
		}
	}

	programOptions := []tea.ProgramOption{}
	if os.Getenv("TG_TUI_ALT_SCREEN") == "1" {
		programOptions = append(programOptions, tea.WithAltScreen())
	}

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
