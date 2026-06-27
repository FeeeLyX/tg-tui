package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

const (
	defaultAppName = "tg-tui"
	defaultCacheDB = "cache.db"
	defaultSession = "telegram-session.json"
	defaultLogFile = "tg-tui.log"
)

type CredentialMode string

const (
	CredentialModeStrict         CredentialMode = "strict"
	CredentialModeSharedDefaults CredentialMode = "shared-defaults"
)

type CredentialSource string

const (
	CredentialSourceEnvironment CredentialSource = "environment"
)

type Config struct {
	TelegramAPIID    int
	TelegramAPIHash  string
	CredentialMode   CredentialMode
	CredentialSource CredentialSource
	DataDir          string
	CachePath        string
	SessionPath      string
	LogPath          string
	Debug            bool
	Verbose          bool
}

func LoadConfig() (Config, error) {
	// Load local .env if present; shell environment remains the source of truth.
	_ = godotenv.Load()

	dataDir, err := resolveDataDir()
	if err != nil {
		return Config{}, err
	}

	apiIDValue := os.Getenv("TG_TUI_API_ID")
	apiHash := os.Getenv("TG_TUI_API_HASH")
	credentialMode, err := resolveCredentialMode(os.Getenv("TG_TUI_CREDENTIAL_MODE"))
	if err != nil {
		return Config{}, err
	}

	var apiID int
	if apiIDValue != "" {
		parsed, parseErr := strconv.Atoi(apiIDValue)
		if parseErr != nil {
			return Config{}, fmt.Errorf("parse TG_TUI_API_ID: %w", parseErr)
		}
		apiID = parsed
	}

	credentialSource, resolvedAPIID, resolvedAPIHash, err := resolveCredentials(credentialMode, apiID, apiHash, apiIDValue != "", apiHash != "")
	if err != nil {
		return Config{}, err
	}

	cachePath := filepath.Join(dataDir, defaultCacheDB)
	sessionPath := filepath.Join(dataDir, defaultSession)
	logPath := filepath.Join(dataDir, defaultLogFile)
	debugEnabled := os.Getenv("TG_TUI_DEBUG") == "1"
	verboseEnabled := debugEnabled || os.Getenv("TG_TUI_VERBOSE") == "1"

	return Config{
		TelegramAPIID:    resolvedAPIID,
		TelegramAPIHash:  resolvedAPIHash,
		CredentialMode:   credentialMode,
		CredentialSource: credentialSource,
		DataDir:          dataDir,
		CachePath:        cachePath,
		SessionPath:      sessionPath,
		LogPath:          logPath,
		Debug:            debugEnabled,
		Verbose:          verboseEnabled,
	}, nil
}

func (c Config) ValidateCredentials() error {
	if c.TelegramAPIID == 0 || c.TelegramAPIHash == "" {
		return errors.New("telegram credentials are missing: set TG_TUI_API_ID and TG_TUI_API_HASH in .env or shell environment")
	}

	return nil
}

func (c Config) CredentialNotice() string {
	if c.CredentialSource != CredentialSourceEnvironment {
		return ""
	}

	return "Using Telegram credentials from environment (.env or shell)."
}

func (c Config) CredentialSummary() string {
	switch c.CredentialSource {
	case CredentialSourceEnvironment:
		return "Credential mode: environment"
	default:
		return ""
	}
}

func resolveCredentialMode(value string) (CredentialMode, error) {
	if value == "" {
		return CredentialModeStrict, nil
	}

	switch CredentialMode(strings.ToLower(strings.TrimSpace(value))) {
	case CredentialModeStrict:
		return CredentialModeStrict, nil
	case CredentialModeSharedDefaults:
		return CredentialModeSharedDefaults, nil
	default:
		return "", fmt.Errorf("invalid TG_TUI_CREDENTIAL_MODE %q: use strict or shared-defaults", value)
	}
}

func resolveCredentials(mode CredentialMode, apiID int, apiHash string, hasAPIID bool, hasAPIHash bool) (CredentialSource, int, string, error) {
	if hasAPIID != hasAPIHash {
		return "", 0, "", errors.New("telegram credentials are incomplete: set both TG_TUI_API_ID and TG_TUI_API_HASH")
	}

	if hasAPIID && hasAPIHash {
		return CredentialSourceEnvironment, apiID, apiHash, nil
	}

	if mode == CredentialModeSharedDefaults {
		return "", 0, "", errors.New("shared-defaults mode is disabled: set TG_TUI_API_ID and TG_TUI_API_HASH in .env or shell environment")
	}

	return "", 0, "", errors.New("telegram credentials are missing: set TG_TUI_API_ID and TG_TUI_API_HASH in .env or shell environment")
}

func resolveDataDir() (string, error) {
	baseDir := os.Getenv("XDG_DATA_HOME")
	if baseDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}

		baseDir = filepath.Join(homeDir, ".local", "share")
	}

	dataDir := filepath.Join(baseDir, defaultAppName)
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return "", fmt.Errorf("create data dir: %w", err)
	}

	return dataDir, nil
}
