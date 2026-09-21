// Package logging builds the process-wide structured logger. Every log
// line is JSON so it can be shipped straight to a log aggregator; the
// level is configurable via LOG_LEVEL for local vs. production use.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// New builds a slog.Logger that writes JSON to stdout at the given level
// (case-insensitive: "debug", "info", "warn", "error"; unknown values
// fall back to "info").
func New(level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	return slog.New(handler)
}
