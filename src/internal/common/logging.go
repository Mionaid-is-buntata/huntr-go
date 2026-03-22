package common

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// SetupLogger configures the global slog logger with JSON output.
// service identifies the calling service (e.g. "scraper", "processor", "web").
// level is a string: "debug", "info", "warn", "error" (defaults to "info").
func SetupLogger(service, level string, w io.Writer) *slog.Logger {
	if w == nil {
		w = os.Stdout
	}

	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: lvl,
	})

	logger := slog.New(handler).With("service", service)
	slog.SetDefault(logger)
	return logger
}
