package processor

import (
	"log/slog"
	"sort"
	"strings"

	"github.com/campbell/huntr-ai/internal/config"
	"github.com/campbell/huntr-ai/internal/models"
)

const (
	TechStackWeight = 30
	DomainWeight    = 25
	LocationWeight  = 20
	SalaryWeight    = 15
)

// matchKeywords counts how many keywords appear in text (case-insensitive).
func matchKeywords(text string, keywords []string) int {
	if text == "" || len(keywords) == 0 {
		return 0
	}
	lower := strings.ToLower(text)
	count := 0
	for _, kw := range keywords {
		if strings.Contains(lower, strings.ToLower(kw)) {
			count++
		}
	}
	return count
}

// matchLocation checks if job location matches any preferred location.
func matchLocation(jobLocation string, preferred []string) bool {
	if jobLocation == "" || len(preferred) == 0 {
		return false
	}
	lower := strings.ToLower(jobLocation)
	for _, loc := range preferred {
		if strings.Contains(lower, strings.ToLower(loc)) {
			return true
		}
	}
	return false
}

// ScoreJob scores a single job against preferences.
func ScoreJob(job *models.Job, prefs config.Preferences) {
	score := 0
	bd := &models.ScoreBreakdown{}

	// Tech stack: 30 pts per match
	techText := job.Title + " " + job.Skills
	techMatches := matchKeywords(techText, prefs.TechStackKeywords)
	techScore := techMatches * TechStackWeight
	score += techScore
	bd.TechStackMatches = techMatches
	bd.TechStackScore = techScore

	// Domain: 25 pts per match
	domainText := job.Title + " " + job.Skills + " " + job.Responsibilities
	domainMatches := matchKeywords(domainText, prefs.DomainKeywords)
	domainScore := domainMatches * DomainWeight
	score += domainScore
	bd.DomainMatches = domainMatches
	bd.DomainScore = domainScore

	// Location: 20 pts (G4: empty locations = no preference = 0 pts)
	if len(prefs.Locations) > 0 {
		if matchLocation(job.Location, prefs.Locations) {
			score += LocationWeight
			bd.LocationMatch = true
			bd.LocationScore = LocationWeight
		}
	}

	// Salary: 15 pts
	if job.SalaryNum != nil && prefs.MinSalary > 0 && *job.SalaryNum >= prefs.MinSalary {
		score += SalaryWeight
		bd.SalaryThreshold = true
		bd.SalaryScore = SalaryWeight
	}

	job.Score = score
	job.ScoreBreakdown = bd
}

// RankJobs sorts jobs by score descending, then title ascending.
func RankJobs(jobs []models.Job) {
	sort.SliceStable(jobs, func(i, j int) bool {
		if jobs[i].Score != jobs[j].Score {
			return jobs[i].Score > jobs[j].Score
		}
		return jobs[i].Title < jobs[j].Title
	})
}

// ScoreJobs scores and ranks all jobs using config preferences.
func ScoreJobs(jobs []models.Job, prefs config.Preferences) []models.Job {
	slog.Info("starting scoring", "count", len(jobs))
	for i := range jobs {
		ScoreJob(&jobs[i], prefs)
	}
	RankJobs(jobs)
	if len(jobs) > 0 {
		slog.Info("scoring complete",
			"min", jobs[len(jobs)-1].Score,
			"max", jobs[0].Score,
		)
	}
	return jobs
}
