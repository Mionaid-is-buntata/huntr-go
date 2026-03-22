package web

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const (
	errorLogPath       = "/data/logs/errors.json"
	scraperErrorPath   = "/data/logs/scraper_errors.json"
	maxErrors          = 100
	errorRetentionDays = 7
)

// ErrorEntry represents a single logged error.
type ErrorEntry struct {
	Timestamp string                 `json:"timestamp"`
	Service   string                 `json:"service"`
	Type      string                 `json:"type"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// ErrorSummary holds aggregated error statistics.
type ErrorSummary struct {
	TotalErrors  int                    `json:"total_errors"`
	ByService    map[string]int         `json:"by_service"`
	ByType       map[string]int         `json:"by_type"`
	RecentErrors []ErrorEntry           `json:"recent_errors"`
}

// ErrorAggregator manages error collection from all services.
type ErrorAggregator struct {
	logPath string
	mu      sync.Mutex
}

var (
	globalAggregator     *ErrorAggregator
	globalAggregatorOnce sync.Once
)

// GetErrorAggregator returns the global error aggregator instance.
func GetErrorAggregator() *ErrorAggregator {
	globalAggregatorOnce.Do(func() {
		globalAggregator = &ErrorAggregator{logPath: errorLogPath}
		os.MkdirAll(filepath.Dir(errorLogPath), 0755)
	})
	return globalAggregator
}

func parseTimestamp(ts string) time.Time {
	if ts == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05.999999", ts)
		if err != nil {
			return time.Time{}
		}
	}
	return t
}

func (ea *ErrorAggregator) loadErrors() []ErrorEntry {
	cutoff := time.Now().AddDate(0, 0, -errorRetentionDays)
	var errors []ErrorEntry

	// Load from errors.json (JSON array)
	if data, err := os.ReadFile(ea.logPath); err == nil {
		var entries []ErrorEntry
		if json.Unmarshal(data, &entries) == nil {
			for _, e := range entries {
				if parseTimestamp(e.Timestamp).After(cutoff) {
					errors = append(errors, e)
				}
			}
		}
	}

	// Load from scraper_errors.json (line-delimited JSON)
	if f, err := os.Open(scraperErrorPath); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			var e ErrorEntry
			if json.Unmarshal([]byte(line), &e) == nil {
				if parseTimestamp(e.Timestamp).After(cutoff) {
					errors = append(errors, e)
				}
			}
		}
	}

	sort.Slice(errors, func(i, j int) bool {
		return errors[i].Timestamp > errors[j].Timestamp
	})
	return errors
}

func (ea *ErrorAggregator) saveErrors(errors []ErrorEntry) {
	if len(errors) > maxErrors {
		errors = errors[len(errors)-maxErrors:]
	}
	data, err := json.MarshalIndent(errors, "", "  ")
	if err != nil {
		slog.Error("error marshalling errors", "error", err)
		return
	}
	os.WriteFile(ea.logPath, data, 0644)
}

// LogError records an error.
func (ea *ErrorAggregator) LogError(service, errorType, message string, details map[string]interface{}) {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	entry := ErrorEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		Service:   service,
		Type:      errorType,
		Message:   message,
		Details:   details,
	}

	errors := ea.loadErrors()
	errors = append(errors, entry)
	ea.saveErrors(errors)
	slog.Error(fmt.Sprintf("[%s] %s: %s", service, errorType, message))
}

// GetRecentErrors returns recent errors, optionally filtered by service.
func (ea *ErrorAggregator) GetRecentErrors(service string, hours, limit int) []ErrorEntry {
	errors := ea.loadErrors()
	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour)

	var filtered []ErrorEntry
	for _, e := range errors {
		if parseTimestamp(e.Timestamp).After(cutoff) {
			if service == "" || e.Service == service {
				filtered = append(filtered, e)
			}
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp > filtered[j].Timestamp
	})

	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered
}

// GetErrorSummary returns aggregated error statistics.
func (ea *ErrorAggregator) GetErrorSummary(hours int) ErrorSummary {
	recent := ea.GetRecentErrors("", hours, maxErrors)

	summary := ErrorSummary{
		TotalErrors: len(recent),
		ByService:   make(map[string]int),
		ByType:      make(map[string]int),
	}

	for _, e := range recent {
		summary.ByService[e.Service]++
		summary.ByType[e.Type]++
	}

	limit := 10
	if len(recent) < limit {
		limit = len(recent)
	}
	summary.RecentErrors = recent[:limit]

	return summary
}

// Convenience functions matching Python API.

func LogError(service, errorType, message string, details map[string]interface{}) {
	GetErrorAggregator().LogError(service, errorType, message, details)
}

func GetRecentErrors(service string, hours, limit int) []ErrorEntry {
	return GetErrorAggregator().GetRecentErrors(service, hours, limit)
}

func GetErrorSummary(hours int) ErrorSummary {
	return GetErrorAggregator().GetErrorSummary(hours)
}
