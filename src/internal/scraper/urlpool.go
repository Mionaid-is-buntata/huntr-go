package scraper

import (
	"log/slog"
	"sync"
)

const (
	// MaxURLAttempts is the maximum number of attempts per URL before rotation.
	MaxURLAttempts = 2
)

// SourcePool holds the URL pool and rotation state for all sources.
var SourcePool = map[string][]string{
	"reed": {
		"https://www.reed.co.uk/jobs/software-developer-jobs",
		"https://www.reed.co.uk/jobs/python-developer-jobs",
		"https://www.reed.co.uk/jobs/full-stack-developer-jobs",
	},
	"adzuna": {
		"https://www.adzuna.co.uk/jobs/search?q=software+developer",
		"https://www.adzuna.co.uk/jobs/search?q=python+developer",
		"https://www.adzuna.co.uk/jobs/search?q=full+stack+developer",
	},
	"technojobs": {
		"https://www.technojobs.co.uk/jobs/python",
		"https://www.technojobs.co.uk/jobs/software-developer",
		"https://www.technojobs.co.uk/jobs/full-stack",
	},
	"linkedin": {
		"https://www.linkedin.com/jobs/search/?keywords=Python+Developer&location=United+Kingdom",
		"https://www.linkedin.com/jobs/search/?keywords=Software+Developer&location=United+Kingdom",
	},
	"indeed": {
		"https://uk.indeed.com/jobs?q=python+developer&l=United+Kingdom",
		"https://uk.indeed.com/jobs?q=software+developer&l=Remote",
	},
	"glassdoor": {
		"https://www.glassdoor.co.uk/Job/python-developer-jobs-SRCH_KO0,16.htm",
		"https://www.glassdoor.co.uk/Job/software-developer-jobs-SRCH_KO0,18.htm",
	},
}

// URLPool manages URL rotation and attempt tracking per source.
type URLPool struct {
	mu       sync.Mutex
	indexes  map[string]int // source -> current URL index
	attempts map[string]int // url -> attempt count
}

// NewURLPool creates a new URL pool tracker.
func NewURLPool() *URLPool {
	return &URLPool{
		indexes:  make(map[string]int),
		attempts: make(map[string]int),
	}
}

// GetURL returns the current URL for a source from the pool.
// Returns the config URL if no pool entry exists.
func (p *URLPool) GetURL(sourceName string, configURL string) string {
	pool, ok := SourcePool[sourceName]
	if !ok || len(pool) == 0 {
		return configURL
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	idx := p.indexes[sourceName]
	if idx >= len(pool) {
		idx = 0
		p.indexes[sourceName] = 0
	}

	return pool[idx]
}

// RecordAttempt records a fetch attempt and returns true if the URL should
// be rotated (max attempts reached with failure).
func (p *URLPool) RecordAttempt(sourceName string, fetchURL string, success bool, jobsFound int) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.attempts[fetchURL]++

	if success && jobsFound > 0 {
		// Success — reset counter
		p.attempts[fetchURL] = 0
		return false
	}

	if p.attempts[fetchURL] >= MaxURLAttempts {
		slog.Info("URL max attempts reached, rotating",
			"source", sourceName, "url", fetchURL, "attempts", p.attempts[fetchURL])
		return true
	}

	slog.Info("URL attempt recorded",
		"source", sourceName, "url", fetchURL,
		"attempt", p.attempts[fetchURL], "max", MaxURLAttempts)
	return false
}

// Rotate advances to the next URL for a source. Returns true if rotation
// succeeded, false if no pool or already exhausted all URLs.
func (p *URLPool) Rotate(sourceName string) bool {
	pool, ok := SourcePool[sourceName]
	if !ok || len(pool) == 0 {
		return false
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.indexes[sourceName] = (p.indexes[sourceName] + 1) % len(pool)
	slog.Info("rotated URL", "source", sourceName, "newIndex", p.indexes[sourceName])
	return true
}

// ResetAttempts resets attempt counts for all URLs of a source.
func (p *URLPool) ResetAttempts(sourceName string) {
	pool, ok := SourcePool[sourceName]
	if !ok {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, u := range pool {
		p.attempts[u] = 0
	}
}
