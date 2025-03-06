package logging

import (
	"log/slog"
	"os"
	"sync"
)

var (
	logger *slog.Logger
	once   sync.Once
)

func Initialize(level string) {
	once.Do(func() {
		var lvl slog.Level
		if err := lvl.UnmarshalText([]byte(level)); err != nil {
			lvl = slog.LevelInfo // default to INFO if invalid level
		}

		opts := &slog.HandlerOptions{
			Level: lvl,
		}
		handler := slog.NewTextHandler(os.Stderr, opts)
		logger = slog.New(handler)
	})
}

func GetLogger() *slog.Logger {
	return logger
}
