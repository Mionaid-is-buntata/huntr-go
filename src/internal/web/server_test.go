package web

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/campbell/huntr-ai/internal/config"
	"github.com/campbell/huntr-ai/internal/models"
)

func testServer(t *testing.T) (*Server, string) {
	t.Helper()
	tmpDir := t.TempDir()
	paths := DataPaths{
		ConfigFile:    filepath.Join(tmpDir, "config", "config.json"),
		CVUploadDir:   filepath.Join(tmpDir, "cv", "cv-latest"),
		CVProfileDir:  filepath.Join(tmpDir, "cv"),
		ScoredDir:     filepath.Join(tmpDir, "jobs", "scored"),
		RawDir:        filepath.Join(tmpDir, "jobs", "raw"),
		NormalisedDir: filepath.Join(tmpDir, "jobs", "normalised"),
		LogsDir:       filepath.Join(tmpDir, "logs"),
		StateDir:      filepath.Join(tmpDir, "state"),
		TemplateDir:   filepath.Join(tmpDir, "templates"),
	}
	paths.EnsureDirs()
	srv := NewServer(paths)
	return srv, tmpDir
}

func TestHealthEndpoint(t *testing.T) {
	srv, _ := testServer(t)
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("health status = %d, want 200", w.Code)
	}
	var body map[string]string
	json.NewDecoder(w.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("status = %q, want %q", body["status"], "ok")
	}
	if body["service"] != "huntr-web" {
		t.Errorf("service = %q, want %q", body["service"], "huntr-web")
	}
}

func TestConfigGetCreatesDefault(t *testing.T) {
	srv, _ := testServer(t)
	req := httptest.NewRequest("GET", "/api/config", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("config GET status = %d, want 200", w.Code)
	}
	var cfg config.Config
	json.NewDecoder(w.Body).Decode(&cfg)
	if len(cfg.JobSources) == 0 {
		t.Error("expected default job sources")
	}
	if cfg.HighScoreThreshold != 70 {
		t.Errorf("threshold = %d, want 70", cfg.HighScoreThreshold)
	}
}

func TestConfigPostAndGet(t *testing.T) {
	srv, _ := testServer(t)

	newCfg := config.Config{
		JobSources:         []config.Source{{Name: "Test", URL: "https://example.com", Enabled: true}},
		Preferences:        config.Preferences{MinSalary: 50000},
		HighScoreThreshold: 80,
	}
	body, _ := json.Marshal(newCfg)

	req := httptest.NewRequest("POST", "/api/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("config POST status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	// Verify round-trip
	req = httptest.NewRequest("GET", "/api/config", nil)
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	var loaded config.Config
	json.NewDecoder(w.Body).Decode(&loaded)
	if loaded.HighScoreThreshold != 80 {
		t.Errorf("round-trip threshold = %d, want 80", loaded.HighScoreThreshold)
	}
}

func TestSourcesCRUD(t *testing.T) {
	srv, _ := testServer(t)

	// GET (creates default config first)
	req := httptest.NewRequest("GET", "/api/sources", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("sources GET = %d", w.Code)
	}

	// PUT disable
	body, _ := json.Marshal(map[string]string{"action": "disable", "name": "Reed"})
	req = httptest.NewRequest("PUT", "/api/sources", bytes.NewReader(body))
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("sources PUT disable = %d, body: %s", w.Code, w.Body.String())
	}

	// DELETE
	body, _ = json.Marshal(map[string]string{"name": "Reed"})
	req = httptest.NewRequest("DELETE", "/api/sources", bytes.NewReader(body))
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("sources DELETE = %d, body: %s", w.Code, w.Body.String())
	}
}

func TestScraperCooldown(t *testing.T) {
	srv, _ := testServer(t)

	// Initially no cooldown
	req := httptest.NewRequest("GET", "/api/scraper/cooldown", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("cooldown GET = %d", w.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["cooldown_active"] != false {
		t.Error("expected no cooldown initially")
	}

	// Trigger manual scrape
	req = httptest.NewRequest("POST", "/api/scraper/trigger", nil)
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("trigger POST = %d, body: %s", w.Code, w.Body.String())
	}

	// Now cooldown should be active
	req = httptest.NewRequest("GET", "/api/scraper/cooldown", nil)
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["cooldown_active"] != true {
		t.Error("expected cooldown to be active after trigger")
	}

	// Second trigger should return 429
	req = httptest.NewRequest("POST", "/api/scraper/trigger", nil)
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != 429 {
		t.Errorf("second trigger = %d, want 429", w.Code)
	}
}

func TestScheduleValidation(t *testing.T) {
	srv, _ := testServer(t)

	// Invalid frequency
	body, _ := json.Marshal(map[string]interface{}{"enabled": true, "frequency": "biweekly", "time": "09:00"})
	req := httptest.NewRequest("POST", "/api/schedule", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("invalid frequency = %d, want 400", w.Code)
	}

	// Valid schedule
	body, _ = json.Marshal(map[string]interface{}{
		"enabled": true, "frequency": "daily", "time": "08:30", "days": []string{},
	})
	req = httptest.NewRequest("POST", "/api/schedule", bytes.NewReader(body))
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("valid schedule = %d, want 200, body: %s", w.Code, w.Body.String())
	}
}

func TestDataClear(t *testing.T) {
	srv, tmpDir := testServer(t)

	// Create some test files
	rawDir := filepath.Join(tmpDir, "jobs", "raw")
	os.WriteFile(filepath.Join(rawDir, "test.json"), []byte("{}"), 0644)

	req := httptest.NewRequest("POST", "/api/data/clear", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("data clear = %d", w.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["removed_count"].(float64) < 1 {
		t.Error("expected at least 1 file removed")
	}
}

func TestCVStatus(t *testing.T) {
	srv, _ := testServer(t)

	req := httptest.NewRequest("GET", "/api/cv/status", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("cv status = %d", w.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "no_cv" {
		t.Errorf("status = %v, want no_cv", resp["status"])
	}
}

func TestDashboardHelpers(t *testing.T) {
	salary := 50000
	jobs := []models.Job{
		{Title: "Go Dev", Company: "Acme", Score: 85, SalaryNum: &salary,
			ScoreBreakdown: &models.ScoreBreakdown{TechStackScore: 60, DomainScore: 25}},
		{Title: "Python Dev", Company: "Corp", Score: 40},
	}

	ctx := GenerateDashboard(jobs)
	if ctx.TotalJobs != 2 {
		t.Errorf("total = %d, want 2", ctx.TotalJobs)
	}
	if ctx.TopScore != 85 {
		t.Errorf("top = %f, want 85", ctx.TopScore)
	}
	if ctx.LowestScore != 40 {
		t.Errorf("lowest = %f, want 40", ctx.LowestScore)
	}
	if ctx.Jobs[0].SalaryNum != 50000 {
		t.Errorf("salary_num = %f, want 50000", ctx.Jobs[0].SalaryNum)
	}
}

func TestCooldownFunctions(t *testing.T) {
	tmp := t.TempDir()
	cooldownFile := filepath.Join(tmp, "cooldown.txt")

	// No file = no cooldown
	active, rem := GetCooldownRemaining(cooldownFile, 30)
	if active || rem != 0 {
		t.Errorf("no file: active=%v rem=%d", active, rem)
	}

	// Write trigger time
	WriteTriggerTime(cooldownFile)

	active, rem = GetCooldownRemaining(cooldownFile, 30)
	if !active {
		t.Error("expected active after write")
	}
	if rem < 29 {
		t.Errorf("remaining = %d, expected ~30", rem)
	}
}

func TestSchedulerCalculation(t *testing.T) {
	next := CalculateNextRun("daily", "09:00", nil)
	if next == nil {
		t.Fatal("expected non-nil for daily")
	}
	if next.Hour() != 9 || next.Minute() != 0 {
		t.Errorf("daily time = %02d:%02d, want 09:00", next.Hour(), next.Minute())
	}

	next = CalculateNextRun("monthly", "10:30", nil)
	if next == nil {
		t.Fatal("expected non-nil for monthly")
	}
	if next.Day() != 1 {
		t.Errorf("monthly day = %d, want 1", next.Day())
	}
}

func TestSourceManager(t *testing.T) {
	cfg := config.Default()
	sources := GetSources(cfg)
	if len(sources) != 6 {
		t.Errorf("sources count = %d, want 6", len(sources))
	}
	// Reed should have a parser
	if !sources[0].HasParser {
		t.Error("Reed should have a parser")
	}

	// Enable/disable
	if !DisableSource(cfg, "Reed") {
		t.Error("DisableSource failed")
	}
	if cfg.JobSources[0].Enabled {
		t.Error("Reed should be disabled")
	}
	if !EnableSource(cfg, "Reed") {
		t.Error("EnableSource failed")
	}

	// Remove
	if !RemoveSource(cfg, "Reed") {
		t.Error("RemoveSource failed")
	}
	if len(cfg.JobSources) != 5 {
		t.Errorf("after remove: %d sources, want 5", len(cfg.JobSources))
	}
}
