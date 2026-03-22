package models

// Job represents a job listing through all pipeline stages.
// Raw fields are populated by scrapers; normalised fields by the processor;
// scored fields by the scorer. JSON tags match the Python data format.
type Job struct {
	// Core fields (set by scrapers)
	Title            string `json:"title"`
	Company          string `json:"company"`
	Location         string `json:"location"`
	Salary           string `json:"salary"`
	Responsibilities string `json:"responsibilities"`
	Skills           string `json:"skills"`
	Link             string `json:"link"`
	Source           string `json:"source"`

	// Normalised fields (set by processor)
	WorkType    string `json:"work_type,omitempty"`
	SalaryNum   *int   `json:"salary_num,omitempty"`
	Description string `json:"description,omitempty"`
	Benefits    string `json:"benefits,omitempty"`

	// Scored fields (set by scorer)
	Score          int             `json:"score,omitempty"`
	ScoreBreakdown *ScoreBreakdown `json:"score_breakdown,omitempty"`
}

// ScoreBreakdown details how a job's relevance score was calculated.
type ScoreBreakdown struct {
	TechStackMatches int  `json:"tech_stack_matches"`
	TechStackScore   int  `json:"tech_stack_score"`
	DomainMatches    int  `json:"domain_matches"`
	DomainScore      int  `json:"domain_score"`
	LocationMatch    bool `json:"location_match"`
	LocationScore    int  `json:"location_score"`
	SalaryThreshold  bool `json:"salary_threshold"`
	SalaryScore      int  `json:"salary_score"`
}
