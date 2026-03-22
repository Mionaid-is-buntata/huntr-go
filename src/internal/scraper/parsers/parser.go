package parsers

import (
	"log/slog"
	"strings"
	"sync"

	"github.com/campbell/huntr-ai/internal/models"
)

// Parser parses job listing HTML into structured jobs.
type Parser interface {
	ParseListings(html string, sourceURL string) ([]models.Job, error)
	Name() string
}

// DetailParser extracts additional fields from a job detail page.
type DetailParser interface {
	ParseDetails(html string) (map[string]string, error)
}

var (
	registry   = make(map[string]Parser)
	detailReg  = make(map[string]DetailParser)
	aliases    = make(map[string]string) // alias -> canonical name
	registryMu sync.RWMutex
)

// Register adds a parser to the registry under the given canonical name.
func Register(name string, p Parser) {
	registryMu.Lock()
	defer registryMu.Unlock()
	key := normalise(name)
	registry[key] = p
	if dp, ok := p.(DetailParser); ok {
		detailReg[key] = dp
	}
}

// RegisterAlias maps an alias name to a canonical parser name.
func RegisterAlias(alias, canonical string) {
	registryMu.Lock()
	defer registryMu.Unlock()
	aliases[normalise(alias)] = normalise(canonical)
}

// GetParser returns the parser for the given source name, checking aliases
// and partial matches. Returns nil, false if not found.
func GetParser(sourceName string) (Parser, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	key := normalise(sourceName)

	// Direct match
	if p, ok := registry[key]; ok {
		return p, true
	}

	// Alias match
	if canonical, ok := aliases[key]; ok {
		if p, ok := registry[canonical]; ok {
			return p, true
		}
	}

	// Partial match: check if any registered name is contained in the source name
	for name, p := range registry {
		if strings.Contains(key, name) {
			return p, true
		}
	}

	slog.Warn("no parser found", "source", sourceName)
	return nil, false
}

// GetDetailParser returns the detail parser for the given source name.
func GetDetailParser(sourceName string) (DetailParser, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	key := normalise(sourceName)

	if dp, ok := detailReg[key]; ok {
		return dp, true
	}
	if canonical, ok := aliases[key]; ok {
		if dp, ok := detailReg[canonical]; ok {
			return dp, true
		}
	}
	return nil, false
}

// ListParsers returns all registered parser names.
func ListParsers() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}

// MergeDetails merges a detail map into a Job, only overwriting empty fields.
func MergeDetails(job *models.Job, details map[string]string) {
	if v, ok := details["description"]; ok && v != "" && job.Description == "" {
		job.Description = v
	}
	if v, ok := details["skills"]; ok && v != "" && job.Skills == "" {
		job.Skills = v
	}
	if v, ok := details["responsibilities"]; ok && v != "" && job.Responsibilities == "" {
		job.Responsibilities = v
	}
	if v, ok := details["benefits"]; ok && v != "" && job.Benefits == "" {
		job.Benefits = v
	}
	if v, ok := details["work_type"]; ok && v != "" && job.WorkType == "" {
		job.WorkType = v
	}
}

// normalise lowercases and strips hyphens, underscores, and spaces for lookup.
func normalise(name string) string {
	s := strings.ToLower(name)
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, " ", "")
	return s
}
