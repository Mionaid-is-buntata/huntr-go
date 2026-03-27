package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/campbell/huntr-ai/internal/common"
	"github.com/campbell/huntr-ai/internal/config"
	"github.com/campbell/huntr-ai/internal/models"
	"github.com/campbell/huntr-ai/internal/processor"
)

const (
	configPath = "/data/config/config.json"
	cvDir      = "/data/cv/cv-latest"
	cvFile     = "/data/cv/cv-latest/cv_uploaded.docx"
	lockFile   = "/data/cv/cv-latest/.processing_lock"
	profileOut = "/data/cv/cv_profile.json"
	rawDir     = "/data/jobs/raw"
	normDir    = "/data/jobs/normalised"
	scoredDir  = "/data/jobs/scored"
	processDir = "/data/cv/cv-processed"
	logFilePath = "/data/logs/processor.log"
	logMaxBytes = 100_000
	logMaxAge   = 24 * time.Hour
)

func pollInterval() int {
	if v := os.Getenv("PROCESSOR_POLL_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 60
}

func processCV() bool {
	if _, err := os.Stat(cvFile); err != nil {
		slog.Debug("no CV file found")
		return false
	}

	if err := processor.CreateLockFile(lockFile); err != nil {
		slog.Info("CV processing already in progress")
		return false
	}
	defer processor.RemoveLockFile(lockFile)

	slog.Info("starting CV processing pipeline")

	// Step 1: Parse CV
	slog.Info("step 1: parsing CV DOCX")
	cvText, err := processor.ParseCVDocx(cvFile)
	if err != nil || cvText == "" {
		slog.Error("failed to parse CV", "error", err)
		return false
	}

	// Step 2: Chunk
	slog.Info("step 2: chunking CV text")
	chunks := processor.ChunkText(cvText, 600, 120)
	if len(chunks) == 0 {
		slog.Error("failed to chunk CV text")
		return false
	}

	// Step 3: Select model and generate embeddings
	slog.Info("step 3: generating embeddings")
	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Warn("config not available for model selection, using defaults", "error", err)
	}
	var llmOverride, embOverride string
	if cfg != nil {
		llmOverride = cfg.CV.LLMModel
		embOverride = cfg.CV.EmbeddingModel
	}
	ollamaClient, err := processor.SelectModel(llmOverride, embOverride)
	if err != nil {
		slog.Error("no Ollama models available", "error", err)
		return false
	}

	embChunks := make([]processor.CVChunkWithEmbedding, len(chunks))
	for i, c := range chunks {
		embChunks[i] = processor.CVChunkWithEmbedding{
			Text:      c.Text,
			Index:     c.Index,
			StartChar: c.StartChar,
			EndChar:   c.EndChar,
		}
	}
	if err := processor.GenerateEmbeddings(embChunks, ollamaClient); err != nil {
		slog.Error("failed to generate embeddings", "error", err)
		return false
	}

	// Step 4: Store in chromem-go
	slog.Info("step 4: storing in vector DB")
	vdb, err := processor.NewVectorDB("")
	if err != nil {
		slog.Error("vector DB init failed", "error", err)
		return false
	}

	collectionName := fmt.Sprintf("cv_%s", time.Now().Format("20060102_150405"))
	meta := map[string]string{
		"filename":    "cv_uploaded.docx",
		"upload_date": time.Now().Format(time.RFC3339),
		"chunk_count": strconv.Itoa(len(embChunks)),
	}
	if err := vdb.StoreCVChunks(collectionName, embChunks, meta); err != nil {
		slog.Error("failed to store CV chunks", "error", err)
		return false
	}

	// Step 5: Extract profile
	slog.Info("step 5: extracting CV profile via Ollama")
	profile, err := processor.ExtractProfile(cvText, ollamaClient)
	if err != nil {
		slog.Warn("profile extraction failed", "error", err)
	} else {
		os.MkdirAll(filepath.Dir(profileOut), 0755)
		if err := processor.SaveProfile(profile, profileOut); err != nil {
			slog.Error("failed to save profile", "error", err)
		}
	}

	// Move CV to processed directory
	os.MkdirAll(processDir, 0755)
	ts := time.Now().Format("20060102_150405")
	dest := filepath.Join(processDir, fmt.Sprintf("cv_processed_%s.docx", ts))
	os.Rename(cvFile, dest)
	slog.Info("CV processing complete", "processed", dest)
	return true
}

// lastProcessedFile tracks the last raw file we processed so we don't re-score it.
var lastProcessedFile string

func loadCVProfile(path string) *models.CVProfile {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var profile models.CVProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		slog.Warn("failed to parse CV profile JSON", "path", path, "error", err)
		return nil
	}
	return &profile
}

func enrichPreferencesWithCVProfile(prefs config.Preferences, profile *models.CVProfile) config.Preferences {
	if profile == nil {
		return prefs
	}
	role := prefs.EffectiveRoleProfile()
	existing := make(map[string]struct{})
	for _, kw := range role.PrimarySkills.Keywords {
		existing[strings.ToLower(kw)] = struct{}{}
	}
	for _, kw := range role.SecondarySkills.Keywords {
		existing[strings.ToLower(kw)] = struct{}{}
	}
	for _, kw := range role.AdjacentSkills.Keywords {
		existing[strings.ToLower(kw)] = struct{}{}
	}

	// Soft enrichment only: add a small number of missing CV skills into adjacent bucket.
	addedSkills := 0
	for _, skill := range profile.Skills {
		key := strings.ToLower(strings.TrimSpace(skill))
		if key == "" {
			continue
		}
		if _, ok := existing[key]; ok {
			continue
		}
		role.AdjacentSkills.Keywords = append(role.AdjacentSkills.Keywords, skill)
		existing[key] = struct{}{}
		addedSkills++
		if addedSkills >= 3 {
			break
		}
	}

	addedDomains := 0
	for _, domain := range profile.Domains {
		d := strings.TrimSpace(domain)
		if d == "" {
			continue
		}
		already := false
		for _, current := range prefs.DomainKeywords {
			if strings.EqualFold(current, d) {
				already = true
				break
			}
		}
		if already {
			continue
		}
		prefs.DomainKeywords = append(prefs.DomainKeywords, d)
		addedDomains++
		if addedDomains >= 2 {
			break
		}
	}

	prefs.RoleProfile = role
	return prefs
}

func processJobs() int {
	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("config not found", "error", err)
		return 1
	}

	// Find latest raw jobs
	entries, err := os.ReadDir(rawDir)
	if err != nil {
		slog.Warn("raw jobs directory not found")
		return 0
	}

	var rawFiles []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			rawFiles = append(rawFiles, filepath.Join(rawDir, e.Name()))
		}
	}
	if len(rawFiles) == 0 {
		slog.Warn("no raw job files found")
		return 0
	}
	sort.Sort(sort.Reverse(sort.StringSlice(rawFiles)))
	latestFile := rawFiles[0]

	// Skip if we already processed this file
	if latestFile == lastProcessedFile {
		slog.Debug("latest raw file already processed, skipping", "file", latestFile)
		return 0
	}

	slog.Info("processing raw jobs", "file", latestFile)
	data, err := os.ReadFile(latestFile)
	if err != nil {
		slog.Error("failed to read raw jobs", "error", err)
		return 1
	}

	var rawJobs []models.Job
	if err := json.Unmarshal(data, &rawJobs); err != nil {
		slog.Error("failed to parse raw jobs", "error", err)
		return 1
	}

	if len(rawJobs) == 0 {
		slog.Info("no jobs to process")
		return 0
	}
	slog.Info("loaded raw jobs", "count", len(rawJobs))

	// Normalise
	slog.Info("step 1: normalising jobs")
	normalised := processor.NormaliseJobs(rawJobs)
	if len(normalised) == 0 {
		slog.Warn("normalisation produced 0 jobs")
		return 0
	}

	// Write normalised
	os.MkdirAll(normDir, 0755)
	ts := time.Now().Format("20060102_150405")
	normFile := filepath.Join(normDir, fmt.Sprintf("jobs_normalised_%s.json", ts))
	normData, err := json.MarshalIndent(normalised, "", "  ")
	if err != nil {
		slog.Error("failed to marshal normalised jobs", "error", err)
		return 1
	}
	if err := os.WriteFile(normFile, normData, 0644); err != nil {
		slog.Error("failed to write normalised jobs", "error", err)
		return 1
	}
	slog.Info("normalised jobs written", "file", normFile, "count", len(normalised))

	prefs := cfg.Preferences
	cvProfile := loadCVProfile(profileOut)
	prefs = enrichPreferencesWithCVProfile(prefs, cvProfile)

	// Score
	slog.Info("step 2: scoring jobs")
	scored := processor.ScoreJobs(normalised, prefs)

	// Write scored
	os.MkdirAll(scoredDir, 0755)
	scoredFile := filepath.Join(scoredDir, fmt.Sprintf("jobs_scored_%s.json", ts))
	scoredData, err := json.MarshalIndent(scored, "", "  ")
	if err != nil {
		slog.Error("failed to marshal scored jobs", "error", err)
		return 1
	}
	if err := os.WriteFile(scoredFile, scoredData, 0644); err != nil {
		slog.Error("failed to write scored jobs", "error", err)
		return 1
	}
	slog.Info("scored jobs written", "file", scoredFile, "count", len(scored))

	lastProcessedFile = latestFile
	return 0
}

func rotateLog() {
	info, err := os.Stat(logFilePath)
	if err != nil {
		return
	}
	if info.Size() <= logMaxBytes {
		return
	}

	archiveDir := filepath.Join(filepath.Dir(logFilePath), "archive")
	os.MkdirAll(archiveDir, 0755)

	ts := time.Now().Format("20060102_150405")
	archivePath := filepath.Join(archiveDir, fmt.Sprintf("processor_%s.log", ts))
	os.Rename(logFilePath, archivePath)
	slog.Info("archived log", "path", archivePath, "size", info.Size())

	entries, _ := os.ReadDir(archiveDir)
	for _, e := range entries {
		if eInfo, err := e.Info(); err == nil && time.Since(eInfo.ModTime()) > logMaxAge {
			os.Remove(filepath.Join(archiveDir, e.Name()))
		}
	}
}

func main() {
	_, logFileHandle := common.SetupLoggerWithFile("processor", os.Getenv("LOG_LEVEL"), logFilePath)
	if logFileHandle != nil {
		defer logFileHandle.Close()
	}
	interval := pollInterval()
	slog.Info("Huntr Processor Service — Starting", "poll_interval", interval)

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

	for {
		rotateLog()
		common.TouchHeartbeat("processor")
		slog.Info("poll cycle started")
		processCV()
		processJobs()
		slog.Info("sleeping", "seconds", interval)
		select {
		case <-ctx.Done():
			slog.Info("shutting down")
			return
		case <-time.After(time.Duration(interval) * time.Second):
		}
	}
}
