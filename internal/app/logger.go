package app

import (
	"fmt"
	"os"
	"sync"
	"time"
)

type Logger struct {
	enabled bool
	file    *os.File
	mu      sync.Mutex
}

func NewLogger(config Config) (*Logger, error) {
	logger := &Logger{enabled: config.Verbose}
	if !logger.enabled {
		return logger, nil
	}

	file, err := os.OpenFile(config.LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	logger.file = file
	logger.Infof("verbose logging enabled")
	logger.Infof("log path: %s", config.LogPath)
	return logger, nil
}

func (l *Logger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}

func (l *Logger) Infof(format string, args ...any) {
	if l == nil || !l.enabled {
		return
	}
	l.write("INFO", format, args...)
}

func (l *Logger) Errorf(format string, args ...any) {
	if l == nil || !l.enabled {
		return
	}
	l.write("ERROR", format, args...)
}

func (l *Logger) write(level string, format string, args ...any) {
	line := fmt.Sprintf("%s [%s] %s\n", time.Now().Format(time.RFC3339), level, fmt.Sprintf(format, args...))

	l.mu.Lock()
	defer l.mu.Unlock()

	_, _ = os.Stderr.WriteString(line)
	if l.file != nil {
		_, _ = l.file.WriteString(line)
	}
}
