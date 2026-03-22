package web

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"

	"github.com/campbell/huntr-ai/internal/config"
	"github.com/campbell/huntr-ai/internal/models"
)

const notificationHistoryFile = "/data/notifications/notified_jobs.json"

// JobUniqueID generates a unique identifier for a job.
func JobUniqueID(job models.Job) string {
	return fmt.Sprintf("%s|%s|%s",
		strings.TrimSpace(job.Title),
		strings.TrimSpace(job.Company),
		strings.TrimSpace(job.Location),
	)
}

// loadNotifiedJobs loads the set of previously notified job IDs.
func loadNotifiedJobs() map[string]bool {
	data, err := os.ReadFile(notificationHistoryFile)
	if err != nil {
		return make(map[string]bool)
	}
	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		slog.Warn("error loading notification history", "error", err)
		return make(map[string]bool)
	}
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

// saveNotifiedJobs persists the notified job IDs.
func saveNotifiedJobs(ids map[string]bool) {
	list := make([]string, 0, len(ids))
	for id := range ids {
		list = append(list, id)
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		slog.Error("error marshalling notification history", "error", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(notificationHistoryFile), 0755); err != nil {
		slog.Error("error creating notification dir", "error", err)
		return
	}
	if err := os.WriteFile(notificationHistoryFile, data, 0644); err != nil {
		slog.Error("error saving notification history", "error", err)
	}
}

// SendEmail sends an email notification for high-scoring jobs.
func SendEmail(jobs []models.Job, cfg *config.Config) error {
	email := os.Getenv("HUNTR_EMAIL")
	password := os.Getenv("HUNTR_EMAIL_PASSWORD")
	recipient := os.Getenv("HUNTR_EMAIL_RECIPIENT")

	if email == "" || password == "" || recipient == "" {
		return fmt.Errorf("email credentials not configured")
	}

	smtpServer := cfg.EmailConfig.SMTPServer
	smtpPort := cfg.EmailConfig.SMTPPort
	if smtpServer == "" {
		smtpServer = "smtp.gmail.com"
	}
	if smtpPort == 0 {
		smtpPort = 587
	}

	subject := fmt.Sprintf("New High-Score Jobs Alert - %d job(s)", len(jobs))
	var body strings.Builder
	body.WriteString("New High-Score Jobs Found:\n\n")
	for i, job := range jobs {
		fmt.Fprintf(&body, "%d. %s at %s\n", i+1, job.Title, job.Company)
		fmt.Fprintf(&body, "   Location: %s\n", job.Location)
		fmt.Fprintf(&body, "   Score: %d\n", job.Score)
		if job.Salary != "" {
			fmt.Fprintf(&body, "   Salary: %s\n", job.Salary)
		}
		fmt.Fprintf(&body, "   Apply: %s\n\n", job.Link)
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		email, recipient, subject, body.String())

	addr := fmt.Sprintf("%s:%d", smtpServer, smtpPort)
	auth := smtp.PlainAuth("", email, password, smtpServer)

	if err := smtp.SendMail(addr, auth, email, []string{recipient}, []byte(msg)); err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	slog.Info("email notification sent", "recipient", recipient, "jobs", len(jobs))
	return nil
}

// NotifyNewJobs checks for new high-score jobs and sends notifications.
// Returns the number of newly notified jobs.
func NotifyNewJobs(jobs []models.Job, cfg *config.Config) int {
	if !cfg.EmailEnabled {
		return 0
	}
	threshold := cfg.HighScoreThreshold
	if threshold == 0 {
		threshold = 70
	}

	var highScore []models.Job
	for _, j := range jobs {
		if j.Score >= threshold {
			highScore = append(highScore, j)
		}
	}
	if len(highScore) == 0 {
		return 0
	}

	notified := loadNotifiedJobs()
	var newJobs []models.Job
	for _, j := range highScore {
		id := JobUniqueID(j)
		if !notified[id] {
			newJobs = append(newJobs, j)
			notified[id] = true
		}
	}
	if len(newJobs) == 0 {
		return 0
	}

	if err := SendEmail(newJobs, cfg); err != nil {
		slog.Error("email notification failed", "error", err)
		return 0
	}
	saveNotifiedJobs(notified)
	slog.Info("notified new high-score jobs", "count", len(newJobs))
	return len(newJobs)
}
