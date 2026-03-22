package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPythonConfig(t *testing.T) {
	// Load the existing Python config.json.template
	templatePath := filepath.Join("..", "..", "..", "docker", "config", "config.json.template")
	cfg, err := Load(templatePath)
	if err != nil {
		t.Fatalf("failed to load config template: %v", err)
	}

	// Verify job_sources
	if len(cfg.JobSources) == 0 {
		t.Fatal("expected at least one job source")
	}
	reed := cfg.JobSources[0]
	if reed.Name != "Reed" {
		t.Errorf("first source name = %q, want %q", reed.Name, "Reed")
	}
	if reed.URL == "" {
		t.Error("first source URL is empty")
	}
	if reed.Dynamic {
		t.Error("Reed should not be dynamic")
	}
	if !reed.Enabled {
		t.Error("Reed should be enabled")
	}

	// Verify preferences
	if len(cfg.Preferences.TechStackKeywords) == 0 {
		t.Error("expected tech_stack_keywords to be populated")
	}
	if len(cfg.Preferences.DomainKeywords) == 0 {
		t.Error("expected domain_keywords to be populated")
	}
	if len(cfg.Preferences.Locations) == 0 {
		t.Error("expected locations to be populated")
	}
	if cfg.Preferences.MinSalary != 100000 {
		t.Errorf("min_salary = %d, want 100000", cfg.Preferences.MinSalary)
	}
	if len(cfg.Preferences.WorkType) == 0 {
		t.Error("expected work_type to be populated")
	}

	// Verify scheduling
	if cfg.Scheduling.Scraper.Frequency != "daily" {
		t.Errorf("frequency = %q, want %q", cfg.Scheduling.Scraper.Frequency, "daily")
	}
	if cfg.Scheduling.Scraper.Time != "09:00" {
		t.Errorf("time = %q, want %q", cfg.Scheduling.Scraper.Time, "09:00")
	}
	if len(cfg.Scheduling.Scraper.Days) != 5 {
		t.Errorf("days count = %d, want 5", len(cfg.Scheduling.Scraper.Days))
	}

	// Verify CV config
	if !cfg.CV.ChunkedProcessing.Enabled {
		t.Error("chunked_processing should be enabled")
	}
	if cfg.CV.ChunkedProcessing.ChunkSize != 600 {
		t.Errorf("chunk_size = %d, want 600", cfg.CV.ChunkedProcessing.ChunkSize)
	}
	if cfg.CV.ChunkedProcessing.ChunkOverlap != 120 {
		t.Errorf("chunk_overlap = %d, want 120", cfg.CV.ChunkedProcessing.ChunkOverlap)
	}
	if cfg.CV.ChunkedProcessing.TopKChunks != 5 {
		t.Errorf("top_k_chunks = %d, want 5", cfg.CV.ChunkedProcessing.TopKChunks)
	}
	if cfg.CV.VectorDB.MaxCollections != 3 {
		t.Errorf("max_collections = %d, want 3", cfg.CV.VectorDB.MaxCollections)
	}
	if !cfg.CV.VectorDB.AutoRotate {
		t.Error("auto_rotate should be true")
	}

	// Verify email
	if !cfg.EmailEnabled {
		t.Error("email_enabled should be true")
	}
	if cfg.EmailConfig.SMTPServer != "smtp.gmail.com" {
		t.Errorf("smtp_server = %q, want %q", cfg.EmailConfig.SMTPServer, "smtp.gmail.com")
	}
	if cfg.EmailConfig.SMTPPort != 587 {
		t.Errorf("smtp_port = %d, want 587", cfg.EmailConfig.SMTPPort)
	}

	// Verify threshold
	if cfg.HighScoreThreshold != 70 {
		t.Errorf("high_score_threshold = %d, want 70", cfg.HighScoreThreshold)
	}
}

func TestSaveAndReload(t *testing.T) {
	cfg := &Config{
		JobSources: []Source{
			{Name: "Test", URL: "https://example.com", Dynamic: false, Enabled: true},
		},
		Preferences: Preferences{
			TechStackKeywords: []string{"Go"},
			MinSalary:         50000,
		},
		HighScoreThreshold: 75,
	}

	tmp := filepath.Join(t.TempDir(), "config.json")
	if err := Save(tmp, cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(tmp)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(loaded.JobSources) != 1 || loaded.JobSources[0].Name != "Test" {
		t.Errorf("round-trip job_sources mismatch")
	}
	if loaded.HighScoreThreshold != 75 {
		t.Errorf("round-trip threshold = %d, want 75", loaded.HighScoreThreshold)
	}

	// Verify file is valid JSON with trailing newline
	data, _ := os.ReadFile(tmp)
	if data[len(data)-1] != '\n' {
		t.Error("saved file should end with newline")
	}
}
