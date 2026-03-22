package scraper

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// DefaultErrorLogPath is the default path for scraper error logs.
	DefaultErrorLogPath = "/data/logs/scraper_errors.json"
)

// ScrapeError matches the web.ErrorEntry JSON format for cross-service compatibility.
type ScrapeError struct {
	Timestamp string                 `json:"timestamp"`
	Service   string                 `json:"service"`
	Type      string                 `json:"type"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// ErrorReporter writes scrape errors as line-delimited JSON.
type ErrorReporter struct {
	path string
	mu   sync.Mutex
}

// NewErrorReporter creates an error reporter writing to the given path.
// If path is empty, uses DefaultErrorLogPath.
func NewErrorReporter(path string) *ErrorReporter {
	if path == "" {
		path = DefaultErrorLogPath
	}
	return &ErrorReporter{path: path}
}

// LogError writes a scrape error to the error log file.
func (r *ErrorReporter) LogError(service, errType, message string, details map[string]interface{}) {
	entry := ScrapeError{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Service:   service,
		Type:      errType,
		Message:   message,
		Details:   details,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		slog.Error("failed to marshal error entry", "err", err)
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Ensure directory exists
	dir := filepath.Dir(r.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		slog.Error("failed to create error log directory", "dir", dir, "err", err)
		return
	}

	f, err := os.OpenFile(r.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		slog.Error("failed to open error log", "path", r.path, "err", err)
		return
	}
	defer f.Close()

	data = append(data, '\n')
	if _, err := f.Write(data); err != nil {
		slog.Error("failed to write error entry", "err", err)
	}
}

// LogFetchError is a convenience for logging fetch failures.
func (r *ErrorReporter) LogFetchError(source, fetchURL string, err error) {
	r.LogError("scraper", "fetch_error", fmt.Sprintf("Failed to fetch %s", source), map[string]interface{}{
		"source": source,
		"url":    fetchURL,
		"error":  err.Error(),
	})
}

// LogParseError is a convenience for logging parse failures.
func (r *ErrorReporter) LogParseError(source string, err error) {
	r.LogError("scraper", "parse_error", fmt.Sprintf("Failed to parse %s", source), map[string]interface{}{
		"source": source,
		"error":  err.Error(),
	})
}
