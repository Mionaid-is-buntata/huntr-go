package web

import (
	"math"
	"time"

	"github.com/campbell/huntr-ai/internal/models"
)

// DashboardContext holds the template rendering context for the dashboard.
type DashboardContext struct {
	Jobs           []FormattedJob `json:"jobs"`
	TotalJobs      int            `json:"total_jobs"`
	TopScore       float64        `json:"top_score"`
	LowestScore    float64        `json:"lowest_score"`
	LastUpdated    string         `json:"last_updated"`
	LastScrapeTime string         `json:"last_scrape_time"`
	PipelineStatus string         `json:"pipeline_status"`
}

// FormattedJob holds a job formatted for display.
type FormattedJob struct {
	Title            string                 `json:"title"`
	Company          string                 `json:"company"`
	Location         string                 `json:"location"`
	Salary           string                 `json:"salary"`
	SalaryNum        float64                `json:"salary_num"`
	WorkType         string                 `json:"work_type"`
	Source           string                 `json:"source"`
	Description      string                 `json:"description"`
	Responsibilities string                 `json:"responsibilities"`
	Benefits         string                 `json:"benefits"`
	Skills           string                 `json:"skills"`
	Link             string                 `json:"link"`
	Score            float64                `json:"score"`
	ScoreBreakdown   map[string]interface{} `json:"score_breakdown"`
}

// sanitizeNum returns 0 for NaN/nil values.
func sanitizeNum(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		if math.IsNaN(val) || math.IsInf(val, 0) {
			return 0
		}
		return val
	case int:
		return float64(val)
	case nil:
		return 0
	default:
		return 0
	}
}

func sanitizeScoreBreakdown(bd *models.ScoreBreakdown) map[string]interface{} {
	if bd == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"tech_stack_matches": bd.TechStackMatches,
		"tech_stack_score":   bd.TechStackScore,
		"domain_matches":     bd.DomainMatches,
		"domain_score":       bd.DomainScore,
		"location_match":     bd.LocationMatch,
		"location_score":     bd.LocationScore,
		"salary_threshold":   bd.SalaryThreshold,
		"salary_score":       bd.SalaryScore,
	}
}

// FormatJobData formats a job for display.
func FormatJobData(job models.Job) FormattedJob {
	resp := job.Responsibilities
	if len(resp) > 200 {
		resp = resp[:200] + "..."
	}

	title := job.Title
	if title == "" {
		title = "Unknown"
	}
	company := job.Company
	if company == "" {
		company = "Unknown"
	}
	location := job.Location
	if location == "" {
		location = "Unknown"
	}
	salary := job.Salary
	if salary == "" {
		salary = "Not specified"
	}
	link := job.Link
	if link == "" {
		link = "#"
	}

	var salaryNum float64
	if job.SalaryNum != nil {
		salaryNum = float64(*job.SalaryNum)
	}

	return FormattedJob{
		Title:            title,
		Company:          company,
		Location:         location,
		Salary:           salary,
		SalaryNum:        salaryNum,
		WorkType:         job.WorkType,
		Source:           job.Source,
		Description:      job.Description,
		Responsibilities: resp,
		Benefits:         job.Benefits,
		Skills:           job.Skills,
		Link:             link,
		Score:            float64(job.Score),
		ScoreBreakdown:   sanitizeScoreBreakdown(job.ScoreBreakdown),
	}
}

// GenerateDashboard builds the template context from scored jobs.
func GenerateDashboard(jobs []models.Job) DashboardContext {
	formatted := make([]FormattedJob, len(jobs))
	for i, j := range jobs {
		formatted[i] = FormatJobData(j)
	}

	var topScore, lowestScore float64
	if len(jobs) > 0 {
		topScore = float64(jobs[0].Score)
		lowestScore = float64(jobs[len(jobs)-1].Score)
	}

	return DashboardContext{
		Jobs:        formatted,
		TotalJobs:   len(jobs),
		TopScore:    topScore,
		LowestScore: lowestScore,
		LastUpdated: time.Now().Format("02/01/2006 15:04"),
	}
}
