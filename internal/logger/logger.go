package logger

import (
	"log/slog"
	"os"
	"strings"
)

var instance *slog.Logger

func init() {
	instance = NewLoggerFromEnv()
}

// NewLoggerFromEnv creates a logger with level from FLARE_DEBUG env (on = debug).
func NewLoggerFromEnv() *slog.Logger {
	isDebug := strings.ToLower(strings.TrimSpace(os.Getenv("FLARE_DEBUG"))) == "on"
	level := slog.LevelInfo
	if isDebug {
		level = slog.LevelDebug
	}
	return NewLogger(level)
}

// NewLogger creates a logger with the given level (for tests or explicit level).
func NewLogger(level slog.Level) *slog.Logger {
	opts := PrettyHandlerOptions{
		SlogOpts: slog.HandlerOptions{Level: level},
	}
	handler := NewPrettyHandler(os.Stdout, opts)
	return slog.New(handler)
}

// SetLogger sets the global logger instance. Used by tests to inject a logger.
func SetLogger(l *slog.Logger) {
	instance = l
}

func GetLogger() *slog.Logger {
	return instance
}
