package scraper

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const sourceStatsFile = "/data/state/source_stats.json"

// SourceStats holds per-source scrape statistics.
type SourceStats struct {
	Source      string `json:"source"`
	JobsFound   int    `json:"jobs_found"`
	ErrorCount  int    `json:"error_count"`
	LastScrapeAt string `json:"last_scrape_at"`
	LastError   string `json:"last_error,omitempty"`
	LastStatus  string `json:"last_status"` // "success", "error", "partial"
}

// StatsRecorder reads and writes per-source scrape statistics.
type StatsRecorder struct {
	path string
	mu   sync.Mutex
}

// NewStatsRecorder creates a StatsRecorder writing to the default path.
func NewStatsRecorder() *StatsRecorder {
	return &StatsRecorder{path: sourceStatsFile}
}

func (sr *StatsRecorder) load() map[string]SourceStats {
	data, err := os.ReadFile(sr.path)
	if err != nil {
		return make(map[string]SourceStats)
	}
	var stats map[string]SourceStats
	if err := json.Unmarshal(data, &stats); err != nil {
		return make(map[string]SourceStats)
	}
	return stats
}

func (sr *StatsRecorder) save(stats map[string]SourceStats) {
	os.MkdirAll(filepath.Dir(sr.path), 0755)
	data, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		slog.Error("failed to marshal source stats", "error", err)
		return
	}
	if err := os.WriteFile(sr.path, data, 0644); err != nil {
		slog.Error("failed to write source stats", "error", err)
	}
}

// RecordSuccess records a successful scrape for a source.
func (sr *StatsRecorder) RecordSuccess(source string, jobCount int) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	stats := sr.load()
	s := stats[source]
	s.Source = source
	s.JobsFound = jobCount
	s.LastScrapeAt = time.Now().Format(time.RFC3339)
	if jobCount > 0 {
		s.LastStatus = "success"
	} else {
		s.LastStatus = "partial"
	}
	stats[source] = s
	sr.save(stats)
}

// RecordError records a failed scrape for a source.
func (sr *StatsRecorder) RecordError(source string, err error) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	stats := sr.load()
	s := stats[source]
	s.Source = source
	s.ErrorCount++
	s.LastScrapeAt = time.Now().Format(time.RFC3339)
	s.LastError = err.Error()
	s.LastStatus = "error"
	s.JobsFound = 0
	stats[source] = s
	sr.save(stats)
}

// GetAll returns all source stats.
func (sr *StatsRecorder) GetAll() map[string]SourceStats {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	return sr.load()
}
