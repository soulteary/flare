package logger

import (
	"log/slog"
	"os"
	"strings"
)

var instance *slog.Logger

func init() {
	isDebug := strings.ToLower(strings.TrimSpace(os.Getenv("FLARE_DEBUG"))) == "on"
	logLevel := slog.LevelInfo
	if isDebug {
		logLevel = slog.LevelDebug
	}

	opts := PrettyHandlerOptions{
		SlogOpts: slog.HandlerOptions{Level: logLevel},
	}
	handler := NewPrettyHandler(os.Stdout, opts)
	instance = slog.New(handler)
}

func GetLogger() *slog.Logger {
	return instance
}
