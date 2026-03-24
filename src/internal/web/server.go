package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/campbell/huntr-ai/internal/config"
	"github.com/campbell/huntr-ai/internal/models"
)

const maxUploadBytes = 10 << 20 // 10MB

// Server holds shared state for the web service.
type Server struct {
	Router     chi.Router
	Paths      DataPaths
	ConfigPath string
	tmpl       *template.Template
}

// NewServer creates a chi router with all routes registered.
func NewServer(paths DataPaths) *Server {
	s := &Server{
		Paths:      paths,
		ConfigPath: paths.ConfigFile,
	}

	// Parse template eagerly; if missing, renderDashboard will retry
	tmplPath := filepath.Join(paths.TemplateDir, "dashboard.html")
	if t, err := template.New("dashboard.html").Funcs(templateFuncs).ParseFiles(tmplPath); err != nil {
		slog.Error("failed to parse template at startup (will retry on request)", "error", err)
	} else {
		s.tmpl = t
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)

	// Health
	r.Get("/health", s.handleHealth)

	// Dashboard
	r.Get("/", s.handleDashboard)

	// API routes
	r.Route("/api", func(r chi.Router) {
		r.Get("/status", s.handleStatus)

		r.Post("/cv/upload", s.handleCVUpload)
		r.Get("/cv/status", s.handleCVStatus)

		r.Get("/config", s.handleConfigGet)
		r.Post("/config", s.handleConfigPost)

		r.Get("/scraper-filters", s.handleScraperFiltersGet)
		r.Post("/scraper-filters", s.handleScraperFiltersPost)

		r.Get("/sources", s.handleSourcesGet)
		r.Post("/sources", s.handleSourcesPost)
		r.Put("/sources", s.handleSourcesPut)
		r.Delete("/sources", s.handleSourcesDelete)
		r.Post("/sources/test", s.handleSourcesTest)
		r.Post("/sources/board/toggle", s.handleBoardToggle)
		r.Get("/sources/stats", s.handleSourceStats)

		r.Get("/collections", s.handleCollectionsGet)
		r.Put("/collections", s.handleCollectionsPut)
		r.Delete("/collections", s.handleCollectionsDelete)

		r.Get("/schedule/next-run", s.handleScheduleNextRun)
		r.Post("/schedule", s.handleSchedulePost)

		r.Get("/errors", s.handleErrorsGet)
		r.Post("/errors", s.handleErrorsPost)

		r.Get("/logs/scraper", s.handleLogsScraper)
		r.Get("/logs/processor", s.handleLogsProcessor)

		r.Post("/scraper/trigger", s.handleScraperTrigger)
		r.Get("/scraper/cooldown", s.handleScraperCooldown)

		r.Post("/data/clear", s.handleDataClear)
	})

	s.Router = r
	return s
}

// loadConfig loads config from disk, creating default if missing.
func (s *Server) loadConfig() (*config.Config, error) {
	cfg, err := config.Load(s.ConfigPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			cfg = config.Default()
			if saveErr := config.Save(s.ConfigPath, cfg); saveErr != nil {
				slog.Error("failed to save default config", "error", saveErr)
			}
			return cfg, nil
		}
		return nil, err
	}
	return cfg, nil
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func noCacheHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
}

// --- Handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok", "service": "huntr-web"})
}

// detectPipelineStatus checks what stage the job pipeline has reached.
func (s *Server) detectPipelineStatus() string {
	if hasFiles(s.Paths.NormalisedDir) {
		return "awaiting_scoring"
	}
	if hasFiles(s.Paths.RawDir) {
		return "awaiting_processing"
	}
	return "no_scrape"
}

// hasFiles returns true if a directory contains at least one regular file.
func hasFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			return true
		}
	}
	return false
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	emptyCtx := DashboardContext{
		Jobs:           []FormattedJob{},
		LastUpdated:    "Never",
		LastScrapeTime: "Never",
	}

	scoredDir := s.Paths.ScoredDir
	entries, err := os.ReadDir(scoredDir)
	if err != nil || len(entries) == 0 {
		emptyCtx.PipelineStatus = s.detectPipelineStatus()
		s.renderDashboard(w, emptyCtx)
		return
	}

	// Find latest scored file
	var scoredFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "jobs_scored_") && strings.HasSuffix(e.Name(), ".json") {
			scoredFiles = append(scoredFiles, filepath.Join(scoredDir, e.Name()))
		}
	}
	if len(scoredFiles) == 0 {
		emptyCtx.PipelineStatus = s.detectPipelineStatus()
		s.renderDashboard(w, emptyCtx)
		return
	}
	sort.Sort(sort.Reverse(sort.StringSlice(scoredFiles)))
	latestFile := scoredFiles[0]

	data, err := os.ReadFile(latestFile)
	if err != nil {
		slog.Error("error reading scored jobs", "error", err)
		writeJSON(w, 500, map[string]string{"error": "Dashboard generation failed"})
		return
	}

	// Parse scored jobs - handle both {"jobs": [...]} and [...]
	var jobs []models.Job
	var wrapped struct{ Jobs []models.Job `json:"jobs"` }
	if json.Unmarshal(data, &wrapped) == nil && wrapped.Jobs != nil {
		jobs = wrapped.Jobs
	} else {
		json.Unmarshal(data, &jobs)
	}

	info, _ := os.Stat(latestFile)
	lastScrape := "Never"
	if info != nil {
		lastScrape = info.ModTime().Format("02/01/2006 15:04")
	}

	ctx := GenerateDashboard(jobs)
	ctx.LastScrapeTime = lastScrape
	s.renderDashboard(w, ctx)
}

// templateFuncs provides template helper functions for the dashboard.
var templateFuncs = template.FuncMap{
	"lower": strings.ToLower,
	"add": func(a, b int) int { return a + b },
	"le": func(a, b int) bool { return a <= b },
	"ge": func(a, b float64) bool { return a >= b },
	"jobsJSON": func(jobs []FormattedJob) template.JS {
		data, err := json.Marshal(jobs)
		if err != nil {
			return template.JS("[]")
		}
		// Escape </ sequences to prevent script injection via job titles/descriptions
		safe := strings.ReplaceAll(string(data), "</", `<\/`)
		return template.JS(safe)
	},
}

func (s *Server) renderDashboard(w http.ResponseWriter, ctx DashboardContext) {
	noCacheHeaders(w)
	if s.tmpl == nil {
		tmplPath := filepath.Join(s.Paths.TemplateDir, "dashboard.html")
		t, err := template.New("dashboard.html").Funcs(templateFuncs).ParseFiles(tmplPath)
		if err != nil {
			slog.Error("error parsing template", "error", err)
			writeJSON(w, 500, map[string]string{"error": "Template not found"})
			return
		}
		s.tmpl = t
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.Execute(w, ctx); err != nil {
		slog.Error("error rendering template", "error", err)
	}
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := GetAllServiceStatus()
	writeJSON(w, 200, status)
}

func (s *Server) handleCVUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		writeJSON(w, 400, map[string]string{"error": "File too large or invalid form data"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "No file provided"})
		return
	}
	defer file.Close()

	if header.Filename == "" {
		writeJSON(w, 400, map[string]string{"error": "No file selected"})
		return
	}
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".docx") {
		writeJSON(w, 400, map[string]string{"error": "Invalid file type. Only .docx files are allowed"})
		return
	}
	if header.Size > maxUploadBytes {
		writeJSON(w, 400, map[string]string{"error": "File too large. Maximum size is 10MB"})
		return
	}

	// Read entire file content
	data, err := io.ReadAll(file)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not read file"})
		return
	}
	if len(data) == 0 {
		writeJSON(w, 400, map[string]string{"error": "File is empty"})
		return
	}

	// Log format info — the processor handles format detection and fallback
	isZip := len(data) >= 4 && data[0] == 0x50 && data[1] == 0x4B
	if isZip {
		slog.Info("CV upload received", "filename", header.Filename, "size", len(data), "format", "docx/zip")
	} else {
		slog.Warn("CV upload: file is not a ZIP/DOCX, processor will attempt plain text fallback",
			"filename", header.Filename, "size", len(data),
			"magic", fmt.Sprintf("%02x %02x %02x %02x", data[0], data[1], data[2], data[3]))
	}

	os.MkdirAll(s.Paths.CVUploadDir, 0755)
	savePath := filepath.Join(s.Paths.CVUploadDir, "cv_uploaded.docx")
	if err := os.WriteFile(savePath, data, 0644); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Upload failed: " + err.Error()})
		return
	}

	slog.Info("CV uploaded", "filename", header.Filename)
	writeJSON(w, 200, map[string]string{
		"status":   "success",
		"message":  "CV uploaded successfully",
		"filename": header.Filename,
		"path":     savePath,
	})
}

func (s *Server) handleCVStatus(w http.ResponseWriter, r *http.Request) {
	profilePath := filepath.Join(s.Paths.CVProfileDir, "cv_profile.json")
	cvFile := filepath.Join(s.Paths.CVUploadDir, "cv_uploaded.docx")

	result := map[string]interface{}{
		"status":      "no_cv",
		"has_cv":      fileExists(cvFile),
		"has_profile": fileExists(profilePath),
	}

	if fileExists(profilePath) {
		data, err := os.ReadFile(profilePath)
		if err == nil {
			var profile interface{}
			json.Unmarshal(data, &profile)
			result["profile"] = profile
		}
		result["status"] = "processed"
	} else if fileExists(cvFile) {
		result["status"] = "uploaded_not_processed"
	}
	writeJSON(w, 200, result)
}

func (s *Server) handleConfigGet(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.loadConfig()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, cfg)
}

func (s *Server) handleConfigPost(w http.ResponseWriter, r *http.Request) {
	var newCfg config.Config
	if err := readJSON(r, &newCfg); err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid JSON"})
		return
	}
	if len(newCfg.JobSources) == 0 {
		writeJSON(w, 400, map[string]string{"error": "Missing required section: job_sources"})
		return
	}
	if err := config.Save(s.ConfigPath, &newCfg); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	slog.Info("configuration updated via API")
	writeJSON(w, 200, map[string]string{"status": "success", "message": "Configuration updated"})
}

func (s *Server) handleScraperFiltersGet(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.loadConfig()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	// Return preferences as scraper filters (scraper_filters is an extension not in the typed config)
	writeJSON(w, 200, map[string]interface{}{
		"min_salary": cfg.Preferences.MinSalary,
		"locations":  cfg.Preferences.Locations,
		"work_type":  cfg.Preferences.WorkType,
	})
}

func (s *Server) handleScraperFiltersPost(w http.ResponseWriter, r *http.Request) {
	var data map[string]interface{}
	if err := readJSON(r, &data); err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid JSON"})
		return
	}

	cfg, err := s.loadConfig()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	if v, ok := data["min_salary"]; ok {
		if f, ok := v.(float64); ok {
			cfg.Preferences.MinSalary = int(f)
		}
	}
	if v, ok := data["locations"]; ok {
		if arr, ok := v.([]interface{}); ok {
			locs := make([]string, 0, len(arr))
			for _, item := range arr {
				if s, ok := item.(string); ok {
					locs = append(locs, s)
				}
			}
			cfg.Preferences.Locations = locs
		}
	}
	if v, ok := data["work_type"]; ok {
		if arr, ok := v.([]interface{}); ok {
			wt := make([]string, 0, len(arr))
			for _, item := range arr {
				if s, ok := item.(string); ok {
					wt = append(wt, s)
				}
			}
			cfg.Preferences.WorkType = wt
		}
	}

	if err := config.Save(s.ConfigPath, cfg); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "success", "message": "Scraper filters saved. Future scrapes will apply these restrictions."})
}

func (s *Server) handleSourcesGet(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.loadConfig()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"sources": GetSources(cfg)})
}

func (s *Server) handleSourcesPost(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Name    string `json:"name"`
		URL     string `json:"url"`
		Dynamic bool   `json:"dynamic"`
		Group   string `json:"group"`
	}
	if err := readJSON(r, &data); err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid JSON"})
		return
	}
	if data.Name == "" || data.URL == "" {
		writeJSON(w, 400, map[string]string{"error": "Name and URL required"})
		return
	}

	cfg, err := s.loadConfig()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	result := AddSource(cfg, data.Name, data.URL, data.Dynamic, data.Group, false)
	if result.Success {
		if err := config.Save(s.ConfigPath, cfg); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Failed to save config: " + err.Error()})
			return
		}
		writeJSON(w, 200, result)
	} else {
		writeJSON(w, 400, result)
	}
}

func (s *Server) handleSourcesPut(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Action string `json:"action"`
		Name   string `json:"name"`
	}
	if err := readJSON(r, &data); err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid JSON"})
		return
	}
	if data.Name == "" {
		writeJSON(w, 400, map[string]string{"error": "Source name required"})
		return
	}

	cfg, err := s.loadConfig()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	var success bool
	switch data.Action {
	case "enable":
		success = EnableSource(cfg, data.Name)
	case "disable":
		success = DisableSource(cfg, data.Name)
	default:
		writeJSON(w, 400, map[string]string{"error": "Invalid action. Use \"enable\" or \"disable\""})
		return
	}

	if success {
		if err := config.Save(s.ConfigPath, cfg); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Failed to save config: " + err.Error()})
			return
		}
		writeJSON(w, 200, map[string]string{"status": "success", "message": "Source \"" + data.Name + "\" " + data.Action + "d"})
	} else {
		writeJSON(w, 404, map[string]string{"error": "Source \"" + data.Name + "\" not found"})
	}
}

func (s *Server) handleSourcesDelete(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &data); err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid JSON"})
		return
	}
	if data.Name == "" {
		writeJSON(w, 400, map[string]string{"error": "Source name required"})
		return
	}

	cfg, err := s.loadConfig()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	if RemoveSource(cfg, data.Name) {
		if err := config.Save(s.ConfigPath, cfg); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Failed to save config: " + err.Error()})
			return
		}
		slog.Info("source removed", "name", data.Name)
		writeJSON(w, 200, map[string]string{"status": "success", "message": "Source \"" + data.Name + "\" removed"})
	} else {
		writeJSON(w, 404, map[string]string{"error": "Source \"" + data.Name + "\" not found"})
	}
}

func (s *Server) handleSourcesTest(w http.ResponseWriter, r *http.Request) {
	var data struct {
		URL     string `json:"url"`
		Dynamic bool   `json:"dynamic"`
	}
	if err := readJSON(r, &data); err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid JSON"})
		return
	}
	if data.URL == "" {
		writeJSON(w, 400, map[string]string{"error": "URL required"})
		return
	}
	// Simple static test — full rod-based dynamic testing will come in Phase 3
	valid := ValidateSourceURL(data.URL)
	writeJSON(w, 200, map[string]interface{}{
		"success":          valid,
		"message":          map[bool]string{true: "Source accessible", false: "Source not accessible"}[valid],
		"recommended_type": "static",
	})
}

func (s *Server) handleBoardToggle(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Board   string `json:"board"`
		Enabled bool   `json:"enabled"`
	}
	if err := readJSON(r, &data); err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid JSON"})
		return
	}
	if data.Board == "" {
		writeJSON(w, 400, map[string]string{"error": "Board ID required"})
		return
	}

	// Board toggle requires a "group" field on sources — we toggle matching sources.
	// Since config.Source doesn't have Group, we load raw JSON and modify it.
	cfg, err := s.loadConfig()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	affected := 0
	for i := range cfg.JobSources {
		if cfg.JobSources[i].Group == data.Board {
			cfg.JobSources[i].Enabled = data.Enabled
			affected++
		}
	}
	if affected > 0 {
		if err := config.Save(s.ConfigPath, cfg); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Failed to save config: " + err.Error()})
			return
		}
	}
	writeJSON(w, 200, map[string]interface{}{"success": true, "affected_count": affected})
}

func (s *Server) handleSourceStats(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile("/data/state/source_stats.json")
	if err != nil {
		writeJSON(w, 200, map[string]interface{}{"stats": map[string]interface{}{}})
		return
	}
	var stats map[string]interface{}
	if err := json.Unmarshal(data, &stats); err != nil {
		writeJSON(w, 200, map[string]interface{}{"stats": map[string]interface{}{}})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"stats": stats})
}

func (s *Server) handleCollectionsGet(w http.ResponseWriter, r *http.Request) {
	cfg, _ := s.loadConfig()
	collections := GetCollections(cfg)
	active := GetActiveCollection(cfg)
	writeJSON(w, 200, map[string]interface{}{
		"collections":       collections,
		"active_collection": active,
	})
}

func (s *Server) handleCollectionsPut(w http.ResponseWriter, r *http.Request) {
	var data struct{ Name string `json:"name"` }
	if err := readJSON(r, &data); err != nil || data.Name == "" {
		writeJSON(w, 400, map[string]string{"error": "Collection name required"})
		return
	}
	cfg, _ := s.loadConfig()
	if SetActiveCollection(cfg, data.Name, s.ConfigPath) {
		writeJSON(w, 200, map[string]string{"status": "success", "message": "Active collection set to \"" + data.Name + "\""})
	} else {
		writeJSON(w, 500, map[string]string{"error": "Failed to set active collection"})
	}
}

func (s *Server) handleCollectionsDelete(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		writeJSON(w, 400, map[string]string{"error": "Collection name required"})
		return
	}
	if DeleteCollection(name) {
		// Clear active collection if it was the deleted one
		cfg, _ := s.loadConfig()
		if cfg != nil && cfg.CV.VectorDB.ActiveCollection == name {
			cfg.CV.VectorDB.ActiveCollection = ""
			config.Save(s.ConfigPath, cfg)
		}
		writeJSON(w, 200, map[string]string{"status": "success", "message": "Collection \"" + name + "\" deleted"})
	} else {
		writeJSON(w, 500, map[string]string{"error": "Failed to delete collection \"" + name + "\""})
	}
}

func (s *Server) handleScheduleNextRun(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.loadConfig()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	sched := cfg.Scheduling.Scraper
	if !sched.Enabled {
		writeJSON(w, 200, map[string]interface{}{
			"next_run": nil, "frequency": nil, "time": nil, "days": []string{},
		})
		return
	}

	next := CalculateNextRun(sched.Frequency, sched.Time, sched.Days)
	var nextStr interface{}
	if next != nil {
		s := next.Format(time.RFC3339)
		nextStr = s
	}

	days := sched.Days
	if sched.Frequency == "daily" {
		days = []string{}
	}

	writeJSON(w, 200, map[string]interface{}{
		"next_run":  nextStr,
		"frequency": sched.Frequency,
		"time":      sched.Time,
		"days":      days,
	})
}

func (s *Server) handleSchedulePost(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Enabled   bool     `json:"enabled"`
		Frequency string   `json:"frequency"`
		Time      string   `json:"time"`
		Days      []string `json:"days"`
	}
	if err := readJSON(r, &data); err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid JSON"})
		return
	}

	// Validate time
	if len(data.Time) < 5 || data.Time[2] != ':' {
		writeJSON(w, 400, map[string]string{"error": "Invalid time format. Use HH:MM"})
		return
	}
	h, _ := strconv.Atoi(data.Time[:2])
	m, _ := strconv.Atoi(data.Time[3:5])
	if h < 0 || h > 23 || m < 0 || m > 59 {
		writeJSON(w, 400, map[string]string{"error": "Invalid time format"})
		return
	}

	// Validate frequency
	switch data.Frequency {
	case "daily", "weekly", "monthly":
	default:
		writeJSON(w, 400, map[string]string{"error": "Invalid frequency. Use daily, weekly, or monthly"})
		return
	}

	validDays := map[string]bool{
		"Monday": true, "Tuesday": true, "Wednesday": true, "Thursday": true,
		"Friday": true, "Saturday": true, "Sunday": true,
	}
	if data.Frequency == "weekly" && data.Enabled {
		if len(data.Days) == 0 {
			writeJSON(w, 400, map[string]string{"error": "At least one day required for weekly schedule"})
			return
		}
		for _, d := range data.Days {
			if !validDays[d] {
				writeJSON(w, 400, map[string]string{"error": "Invalid day: " + d})
				return
			}
		}
	}

	cfg, err := s.loadConfig()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	cfg.Scheduling.Scraper = config.SchedulerConfig{
		Enabled:   data.Enabled,
		Frequency: data.Frequency,
		Time:      data.Time,
		Days:      data.Days,
	}

	if err := config.Save(s.ConfigPath, cfg); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	slog.Info("schedule updated", "enabled", data.Enabled, "frequency", data.Frequency, "time", data.Time)
	writeJSON(w, 200, map[string]string{"status": "success", "message": "Schedule saved successfully"})
}

func (s *Server) handleErrorsGet(w http.ResponseWriter, r *http.Request) {
	hours, _ := strconv.Atoi(r.URL.Query().Get("hours"))
	if hours <= 0 {
		hours = 24
	}
	service := r.URL.Query().Get("service")

	summary := GetErrorSummary(hours)
	recent := GetRecentErrors(service, hours, 50)

	writeJSON(w, 200, map[string]interface{}{
		"summary": summary,
		"errors":  recent,
	})
}

func (s *Server) handleErrorsPost(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Service string                 `json:"service"`
		Type    string                 `json:"type"`
		Message string                 `json:"message"`
		Details map[string]interface{} `json:"details"`
	}
	if err := readJSON(r, &data); err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid JSON"})
		return
	}
	if data.Service == "" {
		data.Service = "web"
	}
	if data.Type == "" {
		data.Type = "client_error"
	}
	LogError(data.Service, data.Type, data.Message, data.Details)
	writeJSON(w, 201, map[string]string{"status": "recorded"})
}

func (s *Server) handleLogsScraper(w http.ResponseWriter, r *http.Request) {
	s.handleLogs(w, r, filepath.Join(s.Paths.LogsDir, "scraper.log"))
}

func (s *Server) handleLogsProcessor(w http.ResponseWriter, r *http.Request) {
	s.handleLogs(w, r, filepath.Join(s.Paths.LogsDir, "processor.log"))
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request, logPath string) {
	linesRequested, _ := strconv.Atoi(r.URL.Query().Get("lines"))
	if linesRequested <= 0 {
		linesRequested = 500
	}
	if linesRequested > 2000 {
		linesRequested = 2000
	}

	info, err := os.Stat(logPath)
	if err != nil {
		writeJSON(w, 200, map[string]interface{}{
			"status": "empty", "content": "", "lines": 0,
			"last_modified": nil, "file_size": 0,
		})
		return
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	allLines := strings.Split(string(data), "\n")
	start := 0
	if len(allLines) > linesRequested {
		start = len(allLines) - linesRequested
	}
	lines := allLines[start:]
	content := strings.Join(lines, "\n")

	writeJSON(w, 200, map[string]interface{}{
		"status":        "ok",
		"content":       content,
		"lines":         len(lines),
		"last_modified": info.ModTime().Format(time.RFC3339),
		"file_size":     info.Size(),
	})
}

func (s *Server) handleScraperTrigger(w http.ResponseWriter, r *http.Request) {
	cooldownFile := filepath.Join(s.Paths.StateDir, "last_manual_scrape.txt")
	active, remaining := GetCooldownRemaining(cooldownFile, DefaultCooldownMinutes)
	if active {
		writeJSON(w, 429, map[string]interface{}{
			"error":                      "Cooldown active",
			"message":                    "Please wait " + strconv.Itoa(remaining) + " more minutes before triggering another manual scrape",
			"cooldown_remaining_minutes": remaining,
		})
		return
	}

	triggerFile := filepath.Join(s.Paths.StateDir, "trigger_manual_scrape")
	if err := WriteTriggerFile(triggerFile); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	WriteTriggerTime(cooldownFile)
	slog.Info("manual scrape triggered via API")
	writeJSON(w, 200, map[string]interface{}{
		"status":           "success",
		"message":          "Manual scrape triggered successfully. Processing will begin shortly.",
		"cooldown_minutes": DefaultCooldownMinutes,
	})
}

func (s *Server) handleScraperCooldown(w http.ResponseWriter, r *http.Request) {
	cooldownFile := filepath.Join(s.Paths.StateDir, "last_manual_scrape.txt")
	active, remaining := GetCooldownRemaining(cooldownFile, DefaultCooldownMinutes)
	writeJSON(w, 200, map[string]interface{}{
		"cooldown_active":            active,
		"cooldown_remaining_minutes": remaining,
	})
}

func (s *Server) handleDataClear(w http.ResponseWriter, r *http.Request) {
	var removed []string
	var errors []string

	dirs := []string{s.Paths.RawDir, s.Paths.NormalisedDir, s.Paths.ScoredDir}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			path := filepath.Join(dir, e.Name())
			if err := os.Remove(path); err != nil {
				errors = append(errors, "Failed to delete "+path+": "+err.Error())
			} else {
				removed = append(removed, path)
			}
		}
	}

	lockFile := filepath.Join(s.Paths.CVUploadDir, ".processing_lock")
	if fileExists(lockFile) {
		if err := os.Remove(lockFile); err != nil {
			errors = append(errors, "Failed to remove lock file: "+err.Error())
		} else {
			removed = append(removed, lockFile)
		}
	}

	slog.Info("clear_run_data", "removed", len(removed), "errors", len(errors))
	writeJSON(w, 200, map[string]interface{}{
		"status":        "success",
		"message":       "Cleared " + strconv.Itoa(len(removed)) + " files",
		"removed_count": len(removed),
		"errors":        errors,
	})
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
