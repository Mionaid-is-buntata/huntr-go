package web

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultCooldownMinutes = 30
	DefaultCooldownFile    = "/data/state/last_manual_scrape.txt"
	TriggerManualFile      = "/data/state/trigger_manual_scrape"
)

// WriteTriggerFile creates the manual-scrape trigger file so the scraper runs on its next poll.
func WriteTriggerFile(triggerFile string) error {
	if triggerFile == "" {
		triggerFile = TriggerManualFile
	}
	if err := os.MkdirAll(filepath.Dir(triggerFile), 0755); err != nil {
		return fmt.Errorf("cooldown: mkdir: %w", err)
	}
	return os.WriteFile(triggerFile, []byte(time.Now().Format(time.RFC3339)), 0644)
}

// GetLastTriggerTime reads the last trigger time from the cooldown file.
// Returns zero time if file is missing, empty, or invalid.
func GetLastTriggerTime(cooldownFile string) time.Time {
	if cooldownFile == "" {
		cooldownFile = DefaultCooldownFile
	}
	data, err := os.ReadFile(cooldownFile)
	if err != nil {
		return time.Time{}
	}
	raw := strings.TrimSpace(string(data))
	if raw == "" {
		return time.Time{}
	}
	// Try RFC3339 first, then Python ISO format fallback
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05.999999", raw)
		if err != nil {
			return time.Time{}
		}
	}
	return t
}

// WriteTriggerTime writes the current time to the cooldown file.
func WriteTriggerTime(cooldownFile string) error {
	if cooldownFile == "" {
		cooldownFile = DefaultCooldownFile
	}
	if err := os.MkdirAll(filepath.Dir(cooldownFile), 0755); err != nil {
		return fmt.Errorf("cooldown: mkdir: %w", err)
	}
	return os.WriteFile(cooldownFile, []byte(time.Now().Format(time.RFC3339)), 0644)
}

// GetCooldownRemaining returns whether cooldown is active and remaining minutes.
// Missing/empty/invalid file is treated as no cooldown.
func GetCooldownRemaining(cooldownFile string, cooldownMinutes int) (active bool, remaining int) {
	if cooldownMinutes <= 0 {
		cooldownMinutes = DefaultCooldownMinutes
	}
	last := GetLastTriggerTime(cooldownFile)
	if last.IsZero() {
		return false, 0
	}
	since := time.Since(last)
	limit := time.Duration(cooldownMinutes) * time.Minute
	if since < limit {
		rem := limit - since
		return true, int(rem.Minutes())
	}
	return false, 0
}
