package logging

import (
	"log/slog"
	"os"
	"sync"
)

var (
	Logger *slog.Logger
	once   sync.Once
)

func Init(level string) {
	once.Do(func() {
		opts := &slog.HandlerOptions{
			Level: parseLevel(level),
		}
		handler := slog.NewTextHandler(os.Stderr, opts)
		Logger = slog.New(handler)
	})
}

// parseLevel converts a string level to slog.Level
func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
