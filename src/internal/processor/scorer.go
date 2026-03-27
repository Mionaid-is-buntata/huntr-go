package processor

import (
	"log/slog"
	"sort"
	"strings"

	"github.com/campbell/huntr-ai/internal/config"
	"github.com/campbell/huntr-ai/internal/models"
)

const (
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

func matchingKeywords(text string, keywords []string) []string {
	if text == "" || len(keywords) == 0 {
		return nil
	}
	lower := strings.ToLower(text)
	seen := make(map[string]struct{}, len(keywords))
	var matches []string
	for _, kw := range keywords {
		kwLower := strings.ToLower(strings.TrimSpace(kw))
		if kwLower == "" {
			continue
		}
		if _, exists := seen[kwLower]; exists {
			continue
		}
		if strings.Contains(lower, kwLower) {
			seen[kwLower] = struct{}{}
			matches = append(matches, kwLower)
		}
	}
	return matches
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

	techText := job.Title + " " + job.Skills + " " + job.Responsibilities + " " + job.Description
	roleProfile := prefs.EffectiveRoleProfile()

	primaryMatches := matchingKeywords(techText, roleProfile.PrimarySkills.Keywords)
	secondaryMatches := matchingKeywords(techText, roleProfile.SecondarySkills.Keywords)
	adjacentMatches := matchingKeywords(techText, roleProfile.AdjacentSkills.Keywords)

	primaryCount := min(len(primaryMatches), roleProfile.PrimarySkills.Cap)
	secondaryCount := min(len(secondaryMatches), roleProfile.SecondarySkills.Cap)
	adjacentCount := min(len(adjacentMatches), roleProfile.AdjacentSkills.Cap)

	primaryScore := primaryCount * roleProfile.PrimarySkills.Weight
	secondaryScore := secondaryCount * roleProfile.SecondarySkills.Weight
	adjacentScore := adjacentCount * roleProfile.AdjacentSkills.Weight
	techScore := primaryScore + secondaryScore + adjacentScore

	excludedCount := matchKeywords(techText, roleProfile.ExcludedSkills)
	excludedPenalty := excludedCount * 5
	if excludedPenalty > techScore/2 {
		excludedPenalty = techScore / 2
	}
	techScore -= excludedPenalty
	score += techScore

	bd.TechStackMatches = primaryCount + secondaryCount + adjacentCount
	bd.TechStackScore = techScore
	bd.PrimaryMatches = primaryCount
	bd.PrimaryScore = primaryScore
	bd.SecondaryMatches = secondaryCount
	bd.SecondaryScore = secondaryScore
	bd.AdjacentMatches = adjacentCount
	bd.AdjacentScore = adjacentScore
	bd.ExcludedPenalty = excludedPenalty

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
