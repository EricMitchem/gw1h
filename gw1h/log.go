package gw1h

import (
	"log/slog"
	"os"
)

func NewLogger() *slog.Logger {
	opts := &slog.HandlerOptions{}

	_, addSource := os.LookupEnv("GW1H_LOG_SOURCE")
	if addSource {
		opts.AddSource = true
	}

	level := os.Getenv("GW1H_LOG_LEVEL")

	switch level {
	case "debug":
		opts.Level = slog.LevelDebug
	case "info":
		opts.Level = slog.LevelInfo
	case "warn":
		opts.Level = slog.LevelWarn
	case "error":
		opts.Level = slog.LevelError
	default:
		opts.Level = slog.LevelInfo
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}
