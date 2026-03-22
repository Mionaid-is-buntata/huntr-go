package web

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/campbell/huntr-ai/internal/config"
)

const maxSources = 50

// SourcesWithParsers is the set of source names that have dedicated parsers.
var SourcesWithParsers = map[string]bool{
	"linkedin": true, "indeed": true, "glassdoor": true, "reed": true,
	"adzuna": true, "technojobs": true, "otta": true, "cvlibrary": true,
	"cv-library": true, "totaljobs": true, "remoteok": true, "remote ok": true,
	"weworkremotely": true, "we work remotely": true, "eustartups": true,
	"eu-startups": true, "eu startups jobs": true, "jobsite": true,
	"interquest": true, "interquestgroup": true, "understandingrecruitment": true,
	"understanding recruitment": true, "roberthalf": true, "robert half": true,
	"swiftra": true,
}

// SourceWithParser extends config.Source with parser availability info.
type SourceWithParser struct {
	config.Source
	HasParser bool   `json:"has_parser"`
	Group     string `json:"group,omitempty"`
}

// normaliseSourceName lowercases and strips hyphens/spaces for parser lookup.
func normaliseSourceName(name string) string {
	return strings.ToLower(strings.NewReplacer(" ", "", "-", "", "_", "").Replace(name))
}

// GetSources returns sources with has_parser info.
func GetSources(cfg *config.Config) []SourceWithParser {
	result := make([]SourceWithParser, 0, len(cfg.JobSources))
	for _, s := range cfg.JobSources {
		result = append(result, SourceWithParser{
			Source:    s,
			HasParser: SourcesWithParsers[normaliseSourceName(s.Name)],
		})
	}
	return result
}

// EnableSource enables a source by name. Returns true if found.
func EnableSource(cfg *config.Config, name string) bool {
	lower := strings.ToLower(name)
	for i := range cfg.JobSources {
		if strings.ToLower(cfg.JobSources[i].Name) == lower {
			cfg.JobSources[i].Enabled = true
			return true
		}
	}
	return false
}

// DisableSource disables a source by name. Returns true if found.
func DisableSource(cfg *config.Config, name string) bool {
	lower := strings.ToLower(name)
	for i := range cfg.JobSources {
		if strings.ToLower(cfg.JobSources[i].Name) == lower {
			cfg.JobSources[i].Enabled = false
			return true
		}
	}
	return false
}

// RemoveSource removes a source by name. Returns true if found and removed.
func RemoveSource(cfg *config.Config, name string) bool {
	lower := strings.ToLower(name)
	for i := range cfg.JobSources {
		if strings.ToLower(cfg.JobSources[i].Name) == lower {
			cfg.JobSources = append(cfg.JobSources[:i], cfg.JobSources[i+1:]...)
			return true
		}
	}
	return false
}

// AddSourceResult holds the result of adding a source.
type AddSourceResult struct {
	Success bool          `json:"success"`
	Message string        `json:"message"`
	Source  *config.Source `json:"source,omitempty"`
}

// AddSource adds a new source to the config.
func AddSource(cfg *config.Config, name, url string, dynamic bool, group string, skipValidation bool) AddSourceResult {
	enabledCount := 0
	for _, s := range cfg.JobSources {
		if s.Enabled {
			enabledCount++
		}
	}
	if enabledCount >= maxSources {
		return AddSourceResult{Success: false, Message: fmt.Sprintf("Maximum %d enabled sources allowed", maxSources)}
	}

	lower := strings.ToLower(name)
	for _, s := range cfg.JobSources {
		if strings.ToLower(s.Name) == lower {
			return AddSourceResult{Success: false, Message: fmt.Sprintf("Source %q already exists", name)}
		}
	}

	if !skipValidation && !ValidateSourceURL(url) {
		return AddSourceResult{Success: false, Message: fmt.Sprintf("URL validation failed: %s", url)}
	}

	newSource := config.Source{
		Name:    name,
		URL:     url,
		Dynamic: dynamic,
		Enabled: true,
		Group:   group,
	}
	cfg.JobSources = append(cfg.JobSources, newSource)
	return AddSourceResult{Success: true, Message: fmt.Sprintf("Source %q added successfully", name), Source: &newSource}
}

// ValidateSourceURL checks if a URL is accessible.
func ValidateSourceURL(url string) bool {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		slog.Warn("URL validation failed", "url", url, "error", err)
		return false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux armv7l) AppleWebKit/537.36")
	resp, err := client.Do(req)
	if err != nil {
		slog.Warn("URL validation failed", "url", url, "error", err)
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}
