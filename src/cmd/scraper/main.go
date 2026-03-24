package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/campbell/huntr-ai/internal/common"
	"github.com/campbell/huntr-ai/internal/config"
	"github.com/campbell/huntr-ai/internal/scraper"
)

const (
	configPath         = "/data/config/config.json"
	triggerFile        = "/data/state/trigger_manual_scrape"
	rawDir             = "/data/jobs/raw"
	logDir             = "/data/logs"
	logFile            = "/data/logs/scraper.log"
	triggerPollSeconds = 60
	logMaxBytes    = 100_000
	logMaxAge      = 24 * time.Hour
)

func pollInterval() int {
	if v := os.Getenv("SCRAPER_POLL_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 1800 // 30 minutes
}

func main() {
	_, logFileHandle := common.SetupLoggerWithFile("scraper", os.Getenv("LOG_LEVEL"), logFile)
	if logFileHandle != nil {
		defer logFileHandle.Close()
	}
	interval := pollInterval()
	slog.Info("Huntr Scraper Service — Starting",
		"poll_interval_s", interval, "poll_interval_m", interval/60)

	// Ensure trigger file parent directory exists
	os.MkdirAll(filepath.Dir(triggerFile), 0o755)

	// Graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		slog.Info("received signal, shutting down", "signal", sig)
		cancel()
	}()

	// Run initial scrape on startup
	slog.Info("running initial scrape on startup")
	runScrape(ctx)
	if ctx.Err() != nil {
		slog.Info("shutting down after initial scrape")
		return
	}

	for {
		// Check for trigger file
		if _, err := os.Stat(triggerFile); err == nil {
			slog.Info("trigger file found, running scrape cycle")
			os.Remove(triggerFile)
			runScrape(ctx)
			continue
		}

		// Wait poll interval, checking for trigger every triggerPollSeconds
		elapsed := 0
		for elapsed < interval {
			select {
			case <-ctx.Done():
				slog.Info("shutting down")
				return
			case <-time.After(time.Duration(triggerPollSeconds) * time.Second):
				elapsed += triggerPollSeconds
			}

			// Check for trigger during wait
			if _, err := os.Stat(triggerFile); err == nil {
				break
			}
		}

		// If trigger appeared, next iteration handles it
		if _, err := os.Stat(triggerFile); err == nil {
			continue
		}

		// Check context before scheduled run
		if ctx.Err() != nil {
			slog.Info("shutting down")
			return
		}

		// Scheduled run
		runScrape(ctx)
	}
}

func runScrape(ctx context.Context) {
	rotateLog()
	common.TouchHeartbeat("scraper")

	slog.Info("============================================================")
	slog.Info("Huntr Scraper Service — Starting job collection")
	slog.Info("============================================================")

	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		return
	}

	s := scraper.New(cfg)
	defer s.Close()

	jobs := s.CollectJobs(ctx, true)

	if len(jobs) == 0 {
		slog.Warn("no jobs collected")
		return
	}

	// Write raw jobs
	os.MkdirAll(rawDir, 0o755)
	ts := time.Now().Format("20060102_150405")
	outFile := filepath.Join(rawDir, fmt.Sprintf("jobs_raw_%s.json", ts))

	data, err := json.MarshalIndent(jobs, "", "  ")
	if err != nil {
		slog.Error("failed to marshal jobs", "error", err)
		return
	}

	if err := os.WriteFile(outFile, data, 0o644); err != nil {
		slog.Error("failed to write jobs", "error", err)
		return
	}

	slog.Info("job collection complete", "total", len(jobs), "file", outFile)
}

func rotateLog() {
	info, err := os.Stat(logFile)
	if err != nil {
		return
	}

	archiveDir := filepath.Join(logDir, "archive")
	os.MkdirAll(archiveDir, 0o755)

	if info.Size() > logMaxBytes {
		ts := time.Now().Format("20060102_150405")
		archivePath := filepath.Join(archiveDir, fmt.Sprintf("scraper_%s.log", ts))
		os.Rename(logFile, archivePath)
		slog.Info("archived log", "path", archivePath, "size", info.Size())

		// Clean old archives
		entries, _ := os.ReadDir(archiveDir)
		type archiveFile struct {
			name string
			mod  time.Time
		}
		var archives []archiveFile
		for _, e := range entries {
			if info, err := e.Info(); err == nil {
				archives = append(archives, archiveFile{e.Name(), info.ModTime()})
			}
		}
		for _, a := range archives {
			if time.Since(a.mod) > logMaxAge {
				os.Remove(filepath.Join(archiveDir, a.name))
			}
		}
	}
}
