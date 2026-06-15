// Package logger настраивает структурированный логгер на базе slog.
package logger

import (
	"log/slog"
	"os"
	"strings"
)

// New создаёт slog.Logger с указанным уровнем (debug | info | warn | error).
func New(level string) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLevel(level),
	})
	return slog.New(handler)
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
