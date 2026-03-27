package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// Config represents the full Huntr configuration, matching config.json.
type Config struct {
	JobSources         []Source        `json:"job_sources"`
	Preferences        Preferences     `json:"preferences"`
	Scheduling         Scheduling      `json:"scheduling"`
	CV                 CVConfig        `json:"cv"`
	EmailEnabled       bool            `json:"email_enabled"`
	EmailConfig        EmailConfig     `json:"email_config"`
	HighScoreThreshold int             `json:"high_score_threshold"`
}

// Source represents a job board source configuration.
type Source struct {
	Name     string   `json:"name"`
	URL      string   `json:"url"`
	Dynamic  bool     `json:"dynamic"`
	Enabled  bool     `json:"enabled"`
	Keywords []string `json:"keywords,omitempty"`
	Group    string   `json:"group,omitempty"`
}

// Preferences holds user scoring and filtering preferences.
type Preferences struct {
	TechStackKeywords []string `json:"tech_stack_keywords"`
	DomainKeywords    []string `json:"domain_keywords"`
	Locations         []string `json:"locations"`
	MinSalary         int      `json:"min_salary"`
	WorkType          []string `json:"work_type"`
	RoleProfile       RoleProfile `json:"role_profile,omitempty"`
}

// SkillGroup configures weighted scoring for a set of related skills.
type SkillGroup struct {
	Keywords []string `json:"keywords"`
	Weight   int      `json:"weight"`
	Cap      int      `json:"cap"`
}

// RoleProfile controls broad, weighted skill matching and query expansion.
type RoleProfile struct {
	PrimarySkills   SkillGroup `json:"primary_skills"`
	SecondarySkills SkillGroup `json:"secondary_skills"`
	AdjacentSkills  SkillGroup `json:"adjacent_skills"`
	ExcludedSkills  []string   `json:"excluded_skills,omitempty"`
	QueryTerms      []string   `json:"query_terms,omitempty"`
}

// Scheduling holds scraper schedule configuration.
type Scheduling struct {
	Scraper SchedulerConfig `json:"scraper"`
}

// SchedulerConfig defines when the scraper runs.
type SchedulerConfig struct {
	Enabled   bool     `json:"enabled"`
	Frequency string   `json:"frequency"`
	Time      string   `json:"time"`
	Days      []string `json:"days"`
}

// CVConfig holds CV processing settings.
type CVConfig struct {
	ChunkedProcessing ChunkConfig  `json:"chunked_processing"`
	VectorDB          VectorConfig `json:"vector_db"`
	LLMModel          string       `json:"llm_model,omitempty"`
	EmbeddingModel    string       `json:"embedding_model,omitempty"`
}

// ChunkConfig defines CV text chunking parameters.
type ChunkConfig struct {
	Enabled      bool `json:"enabled"`
	ChunkSize    int  `json:"chunk_size"`
	ChunkOverlap int  `json:"chunk_overlap"`
	TopKChunks   int  `json:"top_k_chunks"`
}

// VectorConfig defines vector database settings.
type VectorConfig struct {
	MaxCollections   int    `json:"max_collections"`
	AutoRotate       bool   `json:"auto_rotate"`
	ActiveCollection string `json:"active_collection,omitempty"`
}

// EmailConfig holds SMTP connection settings.
type EmailConfig struct {
	SMTPServer string `json:"smtp_server"`
	SMTPPort   int    `json:"smtp_port"`
}

var (
	mu sync.RWMutex
)

// Default returns the default configuration matching the Python config template.
func Default() *Config {
	return &Config{
		JobSources: []Source{
			{Name: "Reed", URL: "https://www.reed.co.uk/jobs/software-developer-jobs-in-london?remote=true", Enabled: true, Group: "general-job-boards"},
			{Name: "Indeed", URL: "https://uk.indeed.com/jobs?q=Software+Engineer+Remote&l=United+Kingdom", Enabled: true, Group: "general-job-boards"},
			{Name: "Adzuna", URL: "https://www.adzuna.co.uk/jobs/search?q=software+engineer+remote", Enabled: true, Group: "general-job-boards"},
			{Name: "Technojobs", URL: "https://www.technojobs.co.uk/jobs/software-engineer", Enabled: true, Group: "tech-specialist-agencies"},
			{Name: "CV-Library", URL: "https://www.cv-library.co.uk/jobs/software-engineer/remote", Enabled: true, Group: "general-job-boards"},
			{Name: "Totaljobs", URL: "https://www.totaljobs.com/jobs/software-engineer/in-united-kingdom?remote=true", Enabled: true, Group: "general-job-boards"},
		},
		Preferences: Preferences{
			TechStackKeywords: []string{"Go", "Platform", "Infrastructure", "DevOps", "Cloud", "Kubernetes", "Docker", "APIs", "Microservices"},
			DomainKeywords:    []string{"FinTech", "E-commerce", "SaaS", "Transport", "Payments", "Healthcare"},
			Locations:         []string{"Remote", "100% Remote", "Hybrid", "London", "Manchester", "UK", "Europe"},
			MinSalary:         100000,
			WorkType:          []string{"100% Remote", "Hybrid"},
			RoleProfile: RoleProfile{
				PrimarySkills: SkillGroup{
					Keywords: []string{"Go", "Golang", "Platform Engineering", "Infrastructure", "DevOps"},
					Weight:   18,
					Cap:      3,
				},
				SecondarySkills: SkillGroup{
					Keywords: []string{"Kubernetes", "Terraform", "AWS", "GCP", "CI/CD", "Observability", "SRE"},
					Weight:   10,
					Cap:      4,
				},
				AdjacentSkills: SkillGroup{
					Keywords: []string{"Python", "React", "TypeScript", "Data Engineering", "Security"},
					Weight:   4,
					Cap:      4,
				},
				ExcludedSkills: []string{"Senior Python Developer", "Django-only"},
				QueryTerms: []string{
					"Go developer remote",
					"Platform engineer remote",
					"DevOps engineer remote",
					"Site reliability engineer remote",
					"Infrastructure engineer remote",
					"Cloud engineer remote",
				},
			},
		},
		Scheduling: Scheduling{
			Scraper: SchedulerConfig{
				Frequency: "daily",
				Time:      "09:00",
				Days:      []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday"},
			},
		},
		CV: CVConfig{
			ChunkedProcessing: ChunkConfig{Enabled: true, ChunkSize: 600, ChunkOverlap: 120, TopKChunks: 5},
			VectorDB:          VectorConfig{MaxCollections: 3, AutoRotate: true},
		},
		EmailEnabled:       true,
		HighScoreThreshold: 70,
	}
}

// Load reads and parses a config.json file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}
	return &cfg, nil
}

// Save writes the config to a JSON file with indentation.
func Save(path string, cfg *Config) error {
	mu.Lock()
	defer mu.Unlock()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("config: write %s: %w", path, err)
	}
	return nil
}

// EffectiveRoleProfile returns a migration-safe role profile.
// If role_profile is not configured, it is derived from tech_stack_keywords.
func (p Preferences) EffectiveRoleProfile() RoleProfile {
	if len(p.RoleProfile.PrimarySkills.Keywords) > 0 ||
		len(p.RoleProfile.SecondarySkills.Keywords) > 0 ||
		len(p.RoleProfile.AdjacentSkills.Keywords) > 0 ||
		len(p.RoleProfile.QueryTerms) > 0 ||
		len(p.RoleProfile.ExcludedSkills) > 0 {
		rp := p.RoleProfile
		if rp.PrimarySkills.Weight <= 0 {
			rp.PrimarySkills.Weight = 12
		}
		if rp.SecondarySkills.Weight <= 0 {
			rp.SecondarySkills.Weight = 7
		}
		if rp.AdjacentSkills.Weight <= 0 {
			rp.AdjacentSkills.Weight = 3
		}
		if rp.PrimarySkills.Cap <= 0 {
			rp.PrimarySkills.Cap = 3
		}
		if rp.SecondarySkills.Cap <= 0 {
			rp.SecondarySkills.Cap = 4
		}
		if rp.AdjacentSkills.Cap <= 0 {
			rp.AdjacentSkills.Cap = 4
		}
		return rp
	}
	return RoleProfile{
		PrimarySkills: SkillGroup{
			Keywords: p.TechStackKeywords,
			Weight:   10,
			Cap:      4,
		},
		SecondarySkills: SkillGroup{
			Keywords: []string{},
			Weight:   6,
			Cap:      3,
		},
		AdjacentSkills: SkillGroup{
			Keywords: []string{},
			Weight:   3,
			Cap:      3,
		},
	}
}
