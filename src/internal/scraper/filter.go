package scraper

import (
	"log/slog"
	"strings"

	"github.com/campbell/huntr-ai/internal/models"
	"github.com/campbell/huntr-ai/internal/scraper/parsers"
)

// ApplyEarlyFilters removes jobs that clearly don't match preferences,
// before the more expensive AI scoring step.
func ApplyEarlyFilters(jobs []models.Job, minSalary int, locations []string, workTypes []string) []models.Job {
	if minSalary <= 0 && len(locations) == 0 && len(workTypes) == 0 {
		return jobs
	}

	before := len(jobs)
	var filtered []models.Job

	for _, job := range jobs {
		if minSalary > 0 && !passesSalaryFilter(job, minSalary) {
			continue
		}
		if len(locations) > 0 && !passesLocationFilter(job, locations) {
			continue
		}
		if len(workTypes) > 0 && !passesWorkTypeFilter(job, workTypes) {
			continue
		}
		filtered = append(filtered, job)
	}

	slog.Info("early filter applied",
		"before", before, "after", len(filtered),
		"removed", before-len(filtered),
		"minSalary", minSalary,
		"locations", locations,
		"workTypes", workTypes)

	return filtered
}

// passesSalaryFilter returns true if the job salary is unknown (pass) or
// meets the minimum. Jobs with no salary are kept (benefit of the doubt).
func passesSalaryFilter(job models.Job, minSalary int) bool {
	if job.Salary == "" {
		return true // no salary info, keep it
	}
	salaryNum := parsers.ExtractSalaryNumber(job.Salary)
	if salaryNum == nil {
		return true // unparsable salary, keep it
	}
	return *salaryNum >= minSalary
}

// passesLocationFilter returns true if the job location contains any of
// the preferred locations (case-insensitive substring match).
func passesLocationFilter(job models.Job, locations []string) bool {
	if job.Location == "" {
		return true // no location info, keep it
	}
	jobLoc := strings.ToLower(job.Location)
	for _, loc := range locations {
		if strings.Contains(jobLoc, strings.ToLower(loc)) {
			return true
		}
	}
	return false
}

// passesWorkTypeFilter returns true if the job work type matches any of
// the preferred work types (case-insensitive).
func passesWorkTypeFilter(job models.Job, workTypes []string) bool {
	if job.WorkType == "" {
		return true // no work type info, keep it
	}
	jobWT := strings.ToLower(job.WorkType)
	for _, wt := range workTypes {
		if strings.EqualFold(jobWT, wt) {
			return true
		}
	}
	return false
}
