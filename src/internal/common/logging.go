package common

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
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

// SetupLoggerWithFile configures logging to both stdout and a file.
// Returns the logger and the open file handle (caller should defer Close).
// If filePath is empty or the file cannot be opened, falls back to stdout only.
func SetupLoggerWithFile(service, level, filePath string) (*slog.Logger, *os.File) {
	if filePath == "" {
		return SetupLogger(service, level, nil), nil
	}
	os.MkdirAll(filepath.Dir(filePath), 0755)
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		slog.Error("failed to open log file, using stdout only", "path", filePath, "error", err)
		return SetupLogger(service, level, nil), nil
	}
	multi := io.MultiWriter(os.Stdout, f)
	return SetupLogger(service, level, multi), f
}

// TouchHeartbeat writes the current timestamp to a heartbeat file for service monitoring.
func TouchHeartbeat(service string) {
	path := fmt.Sprintf("/data/state/%s_heartbeat", service)
	os.MkdirAll(filepath.Dir(path), 0755)
	_ = os.WriteFile(path, []byte(time.Now().Format(time.RFC3339)), 0644)
}
