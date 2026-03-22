package web

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

const serviceTimeout = 30 * time.Minute

// ServiceStatus represents the status of a single service.
type ServiceStatus struct {
	Name         string `json:"name"`
	Status       string `json:"status"`
	LastActivity string `json:"last_activity"`
	Message      string `json:"message"`
}

// latestMtimeInDir returns the most recent file mtime in a directory.
func latestMtimeInDir(dir string) (time.Time, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return time.Time{}, false
	}
	var latest time.Time
	found := false
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if !found || info.ModTime().After(latest) {
			latest = info.ModTime()
			found = true
		}
	}
	return latest, found
}

// CheckServiceStatus checks the status of a service based on log file and data directory activity.
func CheckServiceStatus(name, logFile, dataDir string, extraDirs []string) ServiceStatus {
	status := ServiceStatus{
		Name:   name,
		Status: "unknown",
	}

	var candidates []time.Time

	if info, err := os.Stat(logFile); err == nil {
		candidates = append(candidates, info.ModTime())
	}
	if t, ok := latestMtimeInDir(dataDir); ok {
		candidates = append(candidates, t)
	}
	for _, d := range extraDirs {
		if t, ok := latestMtimeInDir(d); ok {
			candidates = append(candidates, t)
		}
	}

	if len(candidates) == 0 {
		status.Message = "No activity detected"
		return status
	}

	var lastActivity time.Time
	for _, t := range candidates {
		if t.After(lastActivity) {
			lastActivity = t
		}
	}
	status.LastActivity = lastActivity.Format(time.RFC3339)
	age := time.Since(lastActivity)

	switch {
	case age < 5*time.Minute:
		status.Status = "running"
		status.Message = fmt.Sprintf("Active %d minutes ago", int(age.Minutes()))
	case age < serviceTimeout:
		status.Status = "idle"
		status.Message = fmt.Sprintf("Last activity %d minutes ago", int(age.Minutes()))
	default:
		status.Status = "stale"
		status.Message = fmt.Sprintf("No activity for %d days, %d hours", int(age.Hours())/24, int(age.Hours())%24)
	}

	return status
}

// GetAllServiceStatus returns the status of all services.
func GetAllServiceStatus() map[string]ServiceStatus {
	services := map[string]struct {
		log, data string
		extra     []string
	}{
		"scraper":   {"/data/logs/scraper.log", "/data/jobs/raw", nil},
		"processor": {"/data/logs/processor.log", "/data/jobs/scored", []string{"/data/jobs/raw"}},
		"web":       {"/data/logs/web.log", "/data/dashboards", nil},
	}

	result := make(map[string]ServiceStatus, len(services))
	for name, s := range services {
		status := CheckServiceStatus(name, s.log, s.data, s.extra)
		if status.Status == "error" {
			slog.Error("service status check failed", "service", name)
		}
		result[name] = status
	}
	return result
}

// DataPaths holds all file system paths used by the web service.
type DataPaths struct {
	ConfigFile   string
	CVUploadDir  string
	CVProfileDir string
	ScoredDir    string
	RawDir       string
	NormalisedDir string
	LogsDir      string
	StateDir     string
	TemplateDir  string
}

// DefaultDataPaths returns the standard /data/ paths.
func DefaultDataPaths() DataPaths {
	return DataPaths{
		ConfigFile:    "/data/config/config.json",
		CVUploadDir:   "/data/cv/cv-latest",
		CVProfileDir:  "/data/cv",
		ScoredDir:     "/data/jobs/scored",
		RawDir:        "/data/jobs/raw",
		NormalisedDir: "/data/jobs/normalised",
		LogsDir:       "/data/logs",
		StateDir:      "/data/state",
		TemplateDir:   "/app/templates",
	}
}

// EnsureDirs creates all required data directories.
func (p DataPaths) EnsureDirs() error {
	dirs := []string{
		filepath.Dir(p.ConfigFile),
		p.CVUploadDir,
		p.CVProfileDir,
		p.ScoredDir,
		p.RawDir,
		p.NormalisedDir,
		p.LogsDir,
		p.StateDir,
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("ensure dirs: %w", err)
		}
	}
	return nil
}
