package scraper

import (
	"testing"

	"github.com/campbell/huntr-ai/internal/models"
)

func TestApplyEarlyFilters_NoFilters(t *testing.T) {
	jobs := []models.Job{
		{Title: "Go Dev", Salary: "£50,000"},
		{Title: "Python Dev", Salary: "£30,000"},
	}
	result := ApplyEarlyFilters(jobs, 0, nil, nil)
	if len(result) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(result))
	}
}

func TestApplyEarlyFilters_MinSalary(t *testing.T) {
	jobs := []models.Job{
		{Title: "Senior", Salary: "£60,000"},
		{Title: "Junior", Salary: "£25,000"},
		{Title: "Unknown", Salary: ""},
	}
	result := ApplyEarlyFilters(jobs, 40000, nil, nil)
	// Senior (60k passes), Unknown (no salary, kept), Junior (25k filtered)
	if len(result) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(result))
	}
	if result[0].Title != "Senior" {
		t.Errorf("expected Senior first, got %s", result[0].Title)
	}
	if result[1].Title != "Unknown" {
		t.Errorf("expected Unknown second, got %s", result[1].Title)
	}
}

func TestApplyEarlyFilters_Location(t *testing.T) {
	jobs := []models.Job{
		{Title: "A", Location: "London, UK"},
		{Title: "B", Location: "Manchester"},
		{Title: "C", Location: ""},
	}
	result := ApplyEarlyFilters(jobs, 0, []string{"London"}, nil)
	if len(result) != 2 {
		t.Fatalf("expected 2 jobs (London + empty), got %d", len(result))
	}
}

func TestApplyEarlyFilters_WorkType(t *testing.T) {
	jobs := []models.Job{
		{Title: "A", WorkType: "Remote"},
		{Title: "B", WorkType: "On-site"},
		{Title: "C", WorkType: "Hybrid"},
		{Title: "D", WorkType: ""},
	}
	result := ApplyEarlyFilters(jobs, 0, nil, []string{"Remote", "Hybrid"})
	if len(result) != 3 {
		t.Fatalf("expected 3 jobs (Remote + Hybrid + empty), got %d", len(result))
	}
}

func TestApplyEarlyFilters_Combined(t *testing.T) {
	jobs := []models.Job{
		{Title: "Good", Salary: "£60,000", Location: "London", WorkType: "Remote"},
		{Title: "LowPay", Salary: "£20,000", Location: "London", WorkType: "Remote"},
		{Title: "WrongLoc", Salary: "£60,000", Location: "Edinburgh", WorkType: "Remote"},
		{Title: "WrongType", Salary: "£60,000", Location: "London", WorkType: "On-site"},
	}
	result := ApplyEarlyFilters(jobs, 40000, []string{"London"}, []string{"Remote"})
	if len(result) != 1 {
		t.Fatalf("expected 1 job, got %d", len(result))
	}
	if result[0].Title != "Good" {
		t.Errorf("expected Good, got %s", result[0].Title)
	}
}
