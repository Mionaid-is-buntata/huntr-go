package processor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/campbell/huntr-ai/internal/config"
	"github.com/campbell/huntr-ai/internal/models"
)

// --- Normaliser tests ---

func TestNormaliseTitle(t *testing.T) {
	tests := []struct{ in, want string }{
		{"senior dev", "Senior Developer"},
		{"jr eng", "Junior Engineer"},
		{"python developer", "Python Developer"},
		{"", ""},
	}
	for _, tt := range tests {
		got := NormaliseTitle(tt.in)
		if got != tt.want {
			t.Errorf("NormaliseTitle(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseSalary(t *testing.T) {
	tests := []struct {
		in      string
		wantNum *int
	}{
		{"£60k", intPtr(60000)},
		{"60000", intPtr(60000)},
		{"60", intPtr(60000)},
		{"£75,000", intPtr(75000)},
		{"", nil},
		{"Competitive", nil},
	}
	for _, tt := range tests {
		_, got := ParseSalary(tt.in)
		if tt.wantNum == nil {
			if got != nil {
				t.Errorf("ParseSalary(%q) = %d, want nil", tt.in, *got)
			}
		} else if got == nil {
			t.Errorf("ParseSalary(%q) = nil, want %d", tt.in, *tt.wantNum)
		} else if *got != *tt.wantNum {
			t.Errorf("ParseSalary(%q) = %d, want %d", tt.in, *got, *tt.wantNum)
		}
	}
}

func TestStandardiseLocation(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Remote", "Remote"},
		{"work from home", "Remote"},
		{"London, UK", "London"},
		{"Manchester", "Manchester"},
		{"Berlin", "Berlin"},
		{"", ""},
	}
	for _, tt := range tests {
		got := StandardiseLocation(tt.in)
		if got != tt.want {
			t.Errorf("StandardiseLocation(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestStandardiseWorkType(t *testing.T) {
	tests := []struct {
		wt, loc, want string
	}{
		{"remote", "", "Remote"},
		{"", "Remote", "Remote"},
		{"hybrid", "", "Hybrid"},
		{"", "office based", "On-site"},
		{"", "", ""},
	}
	for _, tt := range tests {
		got := StandardiseWorkType(tt.wt, tt.loc)
		if got != tt.want {
			t.Errorf("StandardiseWorkType(%q, %q) = %q, want %q", tt.wt, tt.loc, got, tt.want)
		}
	}
}

func TestRemoveDuplicates(t *testing.T) {
	jobs := []models.Job{
		{Title: "Go Dev", Company: "Acme", Location: "London"},
		{Title: "go dev", Company: "acme", Location: "london"},
		{Title: "Python Dev", Company: "Corp", Location: "Remote"},
	}
	unique := RemoveDuplicates(jobs)
	if len(unique) != 2 {
		t.Errorf("RemoveDuplicates: got %d, want 2", len(unique))
	}
}

func TestNormaliseJobs(t *testing.T) {
	raw := []models.Job{
		{Title: "sr dev", Company: "Acme", Location: "wfh", Salary: "£60k"},
	}
	result := NormaliseJobs(raw)
	if len(result) != 1 {
		t.Fatalf("NormaliseJobs: got %d, want 1", len(result))
	}
	if result[0].Title != "Senior Developer" {
		t.Errorf("title = %q, want Senior Developer", result[0].Title)
	}
	if result[0].Location != "Remote" {
		t.Errorf("location = %q, want Remote", result[0].Location)
	}
	if result[0].SalaryNum == nil || *result[0].SalaryNum != 60000 {
		t.Errorf("salary_num = %v, want 60000", result[0].SalaryNum)
	}
}

// --- Scorer tests ---

func TestScoreJob(t *testing.T) {
	salary := 80000
	job := models.Job{
		Title:    "Python Developer",
		Skills:   "Python Flask React",
		Location: "Remote",
		SalaryNum: &salary,
	}
	prefs := config.Preferences{
		TechStackKeywords: []string{"Python", "Flask", "React"},
		DomainKeywords:    []string{"FinTech"},
		Locations:         []string{"Remote"},
		MinSalary:         50000,
	}

	ScoreJob(&job, prefs)

	// Tech: 3 matches * 30 = 90
	if job.ScoreBreakdown.TechStackScore != 90 {
		t.Errorf("tech_stack_score = %d, want 90", job.ScoreBreakdown.TechStackScore)
	}
	// Domain: 0 matches
	if job.ScoreBreakdown.DomainScore != 0 {
		t.Errorf("domain_score = %d, want 0", job.ScoreBreakdown.DomainScore)
	}
	// Location: match
	if !job.ScoreBreakdown.LocationMatch || job.ScoreBreakdown.LocationScore != 20 {
		t.Errorf("location: match=%v score=%d", job.ScoreBreakdown.LocationMatch, job.ScoreBreakdown.LocationScore)
	}
	// Salary: 80k >= 50k
	if !job.ScoreBreakdown.SalaryThreshold || job.ScoreBreakdown.SalaryScore != 15 {
		t.Errorf("salary: threshold=%v score=%d", job.ScoreBreakdown.SalaryThreshold, job.ScoreBreakdown.SalaryScore)
	}
	// Total: 90 + 0 + 20 + 15 = 125
	if job.Score != 125 {
		t.Errorf("total score = %d, want 125", job.Score)
	}
}

func TestScoreJobEmptyLocations(t *testing.T) {
	job := models.Job{Title: "Dev", Location: "London"}
	prefs := config.Preferences{Locations: []string{}} // G4: empty = no preference = 0 pts

	ScoreJob(&job, prefs)
	if job.ScoreBreakdown.LocationScore != 0 {
		t.Errorf("location_score with empty prefs = %d, want 0", job.ScoreBreakdown.LocationScore)
	}
}

func TestRankJobs(t *testing.T) {
	jobs := []models.Job{
		{Title: "B Job", Score: 50},
		{Title: "A Job", Score: 80},
		{Title: "C Job", Score: 80},
	}
	RankJobs(jobs)
	if jobs[0].Title != "A Job" || jobs[1].Title != "C Job" || jobs[2].Title != "B Job" {
		t.Errorf("ranking order: %s, %s, %s", jobs[0].Title, jobs[1].Title, jobs[2].Title)
	}
}

// --- Chunker tests ---

func TestChunkText(t *testing.T) {
	text := "Hello world. This is a test document with some text that should be chunked properly."
	chunks := ChunkText(text, 40, 10)
	if len(chunks) == 0 {
		t.Fatal("expected at least 1 chunk")
	}
	if chunks[0].Index != 0 {
		t.Errorf("first chunk index = %d", chunks[0].Index)
	}
	if chunks[0].StartChar != 0 {
		t.Errorf("first chunk start = %d", chunks[0].StartChar)
	}
}

func TestChunkTextEmpty(t *testing.T) {
	chunks := ChunkText("", 600, 120)
	if chunks != nil {
		t.Errorf("expected nil for empty text, got %d chunks", len(chunks))
	}
}

// --- Lock file tests ---

func TestLockFile(t *testing.T) {
	tmp := t.TempDir()
	lock := filepath.Join(tmp, ".lock")

	if err := CreateLockFile(lock); err != nil {
		t.Fatalf("CreateLockFile failed: %v", err)
	}
	if _, err := os.Stat(lock); err != nil {
		t.Error("lock file should exist")
	}
	// Second create should fail
	if err := CreateLockFile(lock); err == nil {
		t.Error("expected error on duplicate lock")
	}
	RemoveLockFile(lock)
	if _, err := os.Stat(lock); !os.IsNotExist(err) {
		t.Error("lock file should be removed")
	}
}

// --- CV Parser test ---

func TestParseCVDocxPlainText(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.txt")
	os.WriteFile(path, []byte("This is my CV. I know Python and Go."), 0644)

	text, err := ParseCVDocx(path)
	if err != nil {
		t.Fatalf("ParseCVDocx plain text: %v", err)
	}
	if text == "" {
		t.Error("expected non-empty text")
	}
}

func intPtr(v int) *int { return &v }
