package web

import (
	"log/slog"
	"time"
)

var dayMap = map[string]time.Weekday{
	"Monday":    time.Monday,
	"Tuesday":   time.Tuesday,
	"Wednesday": time.Wednesday,
	"Thursday":  time.Thursday,
	"Friday":    time.Friday,
	"Saturday":  time.Saturday,
	"Sunday":    time.Sunday,
}

// CalculateNextRun calculates the next scheduled run time.
func CalculateNextRun(frequency, timeStr string, days []string) *time.Time {
	now := time.Now()
	hour, minute := parseTime(timeStr)

	switch frequency {
	case "daily":
		next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
		if !next.After(now) {
			next = next.AddDate(0, 0, 1)
		}
		return &next

	case "weekly":
		targetDays := make(map[time.Weekday]bool)
		for _, d := range days {
			if wd, ok := dayMap[d]; ok {
				targetDays[wd] = true
			}
		}
		if len(targetDays) == 0 {
			return nil
		}
		for i := 0; i < 7; i++ {
			candidate := now.AddDate(0, 0, i)
			candidateTime := time.Date(candidate.Year(), candidate.Month(), candidate.Day(),
				hour, minute, 0, 0, now.Location())
			if targetDays[candidateTime.Weekday()] && candidateTime.After(now) {
				return &candidateTime
			}
		}
		// Wrap to next week
		next := now.AddDate(0, 0, 7)
		nextTime := time.Date(next.Year(), next.Month(), next.Day(), hour, minute, 0, 0, now.Location())
		return &nextTime

	case "monthly":
		next := time.Date(now.Year(), now.Month(), 1, hour, minute, 0, 0, now.Location())
		if !next.After(now) {
			next = next.AddDate(0, 1, 0)
		}
		return &next
	}

	return nil
}

// TriggerScraper writes the trigger file for the scraper service.
func TriggerScraper() bool {
	if err := WriteTriggerFile(""); err != nil {
		slog.Error("error triggering scraper", "error", err)
		return false
	}
	slog.Info("scraper triggered via trigger file")
	return true
}

func parseTime(s string) (hour, minute int) {
	hour, minute = 9, 0 // default
	if len(s) >= 5 && s[2] == ':' {
		h := int(s[0]-'0')*10 + int(s[1]-'0')
		m := int(s[3]-'0')*10 + int(s[4]-'0')
		if h >= 0 && h <= 23 && m >= 0 && m <= 59 {
			return h, m
		}
	}
	return
}
