package scraper

import (
	"context"
	"fmt"
	"testing"

	"github.com/campbell/huntr-ai/internal/config"
	"github.com/campbell/huntr-ai/internal/models"
)

// mockFetcher implements PageFetcher for testing.
type mockFetcher struct {
	staticHTML map[string]string // url -> html
	calls      int
}

func (m *mockFetcher) FetchStatic(_ context.Context, url string) (string, error) {
	m.calls++
	if html, ok := m.staticHTML[url]; ok {
		return html, nil
	}
	return "", fmt.Errorf("not found: %s", url)
}

func (m *mockFetcher) FetchDynamic(ctx context.Context, url string) (string, error) {
	return m.FetchStatic(ctx, url)
}

func (m *mockFetcher) Close() {}

func TestCollectJobs_WithMockFetcher(t *testing.T) {
	// HTML that the generic parser can extract jobs from
	html := `<html><body>
		<article>
			<h2><a href="/jobs/123">Go Developer</a></h2>
			<span class="company">Acme Corp</span>
			<span class="location">London</span>
			<span class="salary">£60,000</span>
		</article>
		<article>
			<h2><a href="/jobs/456">Python Dev</a></h2>
			<span class="company">Beta Ltd</span>
			<span class="location">Remote</span>
			<span class="salary">£55,000</span>
		</article>
	</body></html>`

	cfg := &config.Config{
		JobSources: []config.Source{
			{
				Name:    "Generic",
				URL:     "https://example.com/jobs",
				Enabled: true,
			},
		},
		Preferences: config.Preferences{},
	}

	mf := &mockFetcher{
		staticHTML: map[string]string{
			"https://example.com/jobs": html,
		},
	}

	s := &Scraper{
		Fetcher: mf,
		Pool:    NewURLPool(),
		Errors:  NewErrorReporter("/dev/null"),
		Stats:   NewStatsRecorder(),
		Config:  cfg,
	}

	jobs := s.CollectJobs(context.Background(), false)
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	if jobs[0].Title != "Go Developer" {
		t.Errorf("expected 'Go Developer', got %q", jobs[0].Title)
	}
	if jobs[1].Company != "Beta Ltd" {
		t.Errorf("expected 'Beta Ltd', got %q", jobs[1].Company)
	}
}

func TestCollectJobs_FiltersApplied(t *testing.T) {
	html := `<html><body>
		<article>
			<h2><a href="/jobs/1">Senior Dev</a></h2>
			<span class="salary">£80,000</span>
			<span class="location">London</span>
		</article>
		<article>
			<h2><a href="/jobs/2">Junior Dev</a></h2>
			<span class="salary">£25,000</span>
			<span class="location">London</span>
		</article>
	</body></html>`

	cfg := &config.Config{
		JobSources: []config.Source{
			{Name: "Generic", URL: "https://example.com/jobs", Enabled: true},
		},
		Preferences: config.Preferences{
			MinSalary: 50000,
		},
	}

	mf := &mockFetcher{
		staticHTML: map[string]string{
			"https://example.com/jobs": html,
		},
	}

	s := &Scraper{
		Fetcher: mf,
		Pool:    NewURLPool(),
		Errors:  NewErrorReporter("/dev/null"),
		Stats:   NewStatsRecorder(),
		Config:  cfg,
	}

	jobs := s.CollectJobs(context.Background(), false)
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job after salary filter, got %d", len(jobs))
	}
	if jobs[0].Title != "Senior Dev" {
		t.Errorf("expected 'Senior Dev', got %q", jobs[0].Title)
	}
}

func TestCollectJobs_NoEnabledSources(t *testing.T) {
	cfg := &config.Config{
		JobSources: []config.Source{
			{Name: "Reed", URL: "https://example.com", Enabled: false},
		},
	}

	s := &Scraper{
		Fetcher: &mockFetcher{},
		Pool:    NewURLPool(),
		Errors:  NewErrorReporter("/dev/null"),
		Stats:   NewStatsRecorder(),
		Config:  cfg,
	}

	jobs := s.CollectJobs(context.Background(), false)
	if len(jobs) != 0 {
		t.Fatalf("expected 0 jobs, got %d", len(jobs))
	}
}

func TestCollectJobs_ContextCancellation(t *testing.T) {
	cfg := &config.Config{
		JobSources: []config.Source{
			{Name: "Generic", URL: "https://example.com/jobs", Enabled: true},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	s := &Scraper{
		Fetcher: &mockFetcher{},
		Pool:    NewURLPool(),
		Errors:  NewErrorReporter("/dev/null"),
		Stats:   NewStatsRecorder(),
		Config:  cfg,
	}

	jobs := s.CollectJobs(ctx, false)
	if len(jobs) != 0 {
		t.Fatalf("expected 0 jobs on cancelled context, got %d", len(jobs))
	}
}

func TestEnabledSources(t *testing.T) {
	cfg := &config.Config{
		JobSources: []config.Source{
			{Name: "A", Enabled: true},
			{Name: "B", Enabled: false},
			{Name: "C", Enabled: true},
		},
	}
	enabled := enabledSources(cfg)
	if len(enabled) != 2 {
		t.Fatalf("expected 2 enabled sources, got %d", len(enabled))
	}
}

// Ensure models.Job is used (prevents unused import).
var _ = models.Job{}
