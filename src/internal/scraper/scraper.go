package scraper

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/campbell/huntr-ai/internal/config"
	"github.com/campbell/huntr-ai/internal/models"
	"github.com/campbell/huntr-ai/internal/scraper/parsers"
)

const (
	// RateLimitDelay is the base delay between requests (seconds).
	RateLimitDelay = 3.0
)

// Scraper orchestrates job collection from all configured sources.
type Scraper struct {
	Fetcher  PageFetcher
	Pool     *URLPool
	Errors   *ErrorReporter
	Stats    *StatsRecorder
	Config   *config.Config
}

// New creates a new Scraper with real dependencies.
func New(cfg *config.Config) *Scraper {
	return &Scraper{
		Fetcher: NewFetcher(),
		Pool:    NewURLPool(),
		Errors:  NewErrorReporter(""),
		Stats:   NewStatsRecorder(),
		Config:  cfg,
	}
}

// CollectJobs fetches job listings from all enabled sources, applies early
// filters, and optionally fetches detail pages. Sequential processing
// (no goroutines per source) matches Python production behaviour.
func (s *Scraper) CollectJobs(ctx context.Context, fetchDetails bool) []models.Job {
	sources := enabledSources(s.Config)
	if len(sources) == 0 {
		slog.Warn("no enabled sources found")
		return nil
	}

	slog.Info("starting job collection", "sources", len(sources))

	// Extract filter preferences
	prefs := s.Config.Preferences
	minSalary := prefs.MinSalary
	locations := prefs.Locations
	workTypes := prefs.WorkType

	if minSalary > 0 || len(locations) > 0 || len(workTypes) > 0 {
		slog.Info("early filtering enabled",
			"minSalary", minSalary, "locations", locations, "workTypes", workTypes)
	}

	var allJobs []models.Job

	for _, src := range sources {
		if err := ctx.Err(); err != nil {
			slog.Info("context cancelled, stopping collection")
			break
		}

		jobs := s.collectFromSource(ctx, src, prefs, minSalary, locations, workTypes, fetchDetails)
		allJobs = append(allJobs, jobs...)

		// Rate limit between sources
		delay := addJitter(RateLimitDelay)
		select {
		case <-time.After(time.Duration(delay * float64(time.Second))):
		case <-ctx.Done():
			slog.Info("context cancelled during source cooldown")
			return allJobs
		}
	}

	slog.Info("job collection complete", "total", len(allJobs))
	return allJobs
}

func (s *Scraper) collectFromSource(
	ctx context.Context,
	src config.Source,
	prefs config.Preferences,
	minSalary int,
	locations, workTypes []string,
	fetchDetails bool,
) []models.Job {
	sourceName := strings.ToLower(src.Name)

	parser, ok := parsers.GetParser(sourceName)
	if !ok {
		slog.Warn("no parser for source, skipping", "source", src.Name)
		return nil
	}

	// Reset URL attempts for this source
	s.Pool.ResetAttempts(sourceName)

	// Determine max rotations
	pool, hasPool := SourcePool[sourceName]
	maxRotations := 1
	if hasPool {
		maxRotations = len(pool)
	}

	currentURL := src.URL
	tried := make(map[string]bool)
	var jobsForSource []models.Job

	for rotation := 0; rotation < maxRotations; rotation++ {
		if err := ctx.Err(); err != nil {
			return nil
		}

		// Get URL from pool if no config URL
		if currentURL == "" {
			currentURL = s.Pool.GetURL(sourceName, "")
			if currentURL == "" {
				slog.Warn("no URL available for source", "source", src.Name)
				break
			}
		}

		if tried[currentURL] {
			s.Pool.Rotate(sourceName)
			currentURL = s.Pool.GetURL(sourceName, "")
			continue
		}
		tried[currentURL] = true

		// Build search URL with source-specific keywords. If source keywords are
		// not explicitly configured, derive broad rotating terms from role profile.
		searchKeywords := sourceKeywordsForSearch(src, prefs, sourceName)
		searchURL := BuildSearchURL(currentURL, sourceName, searchKeywords)
		slog.Info("fetching source", "source", src.Name,
			"url", searchURL, "rotation", rotation+1, "max", maxRotations)

		// Fetch HTML
		var html string
		var err error
		if src.Dynamic {
			html, err = s.Fetcher.FetchDynamic(ctx, searchURL)
		} else {
			html, err = s.Fetcher.FetchStatic(ctx, searchURL)
		}

		if err != nil {
			slog.Warn("fetch failed", "source", src.Name, "error", err)
			s.Errors.LogFetchError(src.Name, searchURL, err)
			if s.Pool.RecordAttempt(sourceName, searchURL, false, 0) {
				s.Pool.Rotate(sourceName)
				currentURL = s.Pool.GetURL(sourceName, "")
			}
			continue
		}

		// Parse listings
		jobs, err := parser.ParseListings(html, searchURL)
		if err != nil {
			slog.Warn("parse failed", "source", src.Name, "error", err)
			s.Errors.LogParseError(src.Name, err)
			if s.Pool.RecordAttempt(sourceName, searchURL, false, 0) {
				s.Pool.Rotate(sourceName)
				currentURL = s.Pool.GetURL(sourceName, "")
			}
			continue
		}

		// Apply early filters
		jobs = ApplyEarlyFilters(jobs, minSalary, locations, workTypes)

		if len(jobs) == 0 {
			slog.Warn("no jobs after filtering", "source", src.Name)
			if s.Pool.RecordAttempt(sourceName, searchURL, true, 0) {
				s.Pool.Rotate(sourceName)
				currentURL = s.Pool.GetURL(sourceName, "")
			}
			continue
		}

		// Success
		s.Pool.RecordAttempt(sourceName, searchURL, true, len(jobs))
		jobsForSource = append(jobsForSource, jobs...)
		slog.Info("collected jobs", "source", src.Name, "count", len(jobs))
		break
	}

	// Record per-source stats
	if len(jobsForSource) > 0 {
		s.Stats.RecordSuccess(src.Name, len(jobsForSource))
	} else {
		s.Stats.RecordError(src.Name, fmt.Errorf("no jobs collected"))
	}

	// Fetch detail pages
	if fetchDetails && len(jobsForSource) > 0 {
		jobsForSource = s.fetchDetails(ctx, src, jobsForSource)
	}

	return jobsForSource
}

func (s *Scraper) fetchDetails(ctx context.Context, src config.Source, jobs []models.Job) []models.Job {
	dp, ok := parsers.GetDetailParser(strings.ToLower(src.Name))
	if !ok {
		return jobs
	}

	slog.Info("fetching details", "source", src.Name, "count", len(jobs))

	for i := range jobs {
		if err := ctx.Err(); err != nil {
			break
		}

		if jobs[i].Link == "" || jobs[i].Link == "Unknown" {
			continue
		}

		var html string
		var err error
		if src.Dynamic {
			html, err = s.Fetcher.FetchDynamic(ctx, jobs[i].Link)
		} else {
			html, err = s.Fetcher.FetchStatic(ctx, jobs[i].Link)
		}

		if err != nil {
			slog.Debug("detail fetch failed", "title", jobs[i].Title, "error", err)
			continue
		}

		details, err := dp.ParseDetails(html)
		if err != nil {
			slog.Debug("detail parse failed", "title", jobs[i].Title, "error", err)
			continue
		}

		parsers.MergeDetails(&jobs[i], details)

		// Rate limit between detail fetches
		delay := addJitter(RateLimitDelay)
		select {
		case <-time.After(time.Duration(delay * float64(time.Second))):
		case <-ctx.Done():
			return jobs
		}
	}

	return jobs
}

// Close releases resources (browser instances, etc).
func (s *Scraper) Close() {
	s.Fetcher.Close()
}

// enabledSources returns only enabled sources from the config.
func enabledSources(cfg *config.Config) []config.Source {
	var enabled []config.Source
	for _, src := range cfg.JobSources {
		if src.Enabled {
			enabled = append(enabled, src)
		}
	}
	return enabled
}

func sourceKeywordsForSearch(src config.Source, prefs config.Preferences, sourceName string) []string {
	if len(src.Keywords) > 0 {
		return src.Keywords
	}
	roleProfile := prefs.EffectiveRoleProfile()
	terms := roleProfile.QueryTerms
	if len(terms) == 0 {
		terms = append(terms, roleProfile.PrimarySkills.Keywords...)
		terms = append(terms, roleProfile.SecondarySkills.Keywords...)
	}
	if len(terms) == 0 {
		return nil
	}
	return rotatedKeywords(terms, sourceName, 3)
}

func rotatedKeywords(terms []string, sourceName string, take int) []string {
	if len(terms) == 0 || take <= 0 {
		return nil
	}
	offset := (time.Now().YearDay() + len(sourceName)) % len(terms)
	out := make([]string, 0, min(take, len(terms)))
	for i := 0; i < take && i < len(terms); i++ {
		out = append(out, terms[(offset+i)%len(terms)])
	}
	return out
}
