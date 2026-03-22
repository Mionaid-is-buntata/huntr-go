package parsers

import (
	"testing"
)

const testReedHTML = `
<html><body>
<div class="jobCard">
  <a data-qa="job-card-title" href="/jobs/senior-go-developer/12345">Senior Go Developer</a>
  <div data-qa="job-posted-by">
    <a class="gtmJobListingPostedBy" href="/recruiter/acme">Acme Corp</a>
  </div>
  <li data-qa="job-metadata-location">London, UK</li>
  <li data-qa="job-metadata-salary">£80,000 - £100,000</li>
</div>
<div class="jobCard">
  <a data-qa="job-card-title" href="/jobs/python-dev/67890">Python Developer</a>
  <div data-qa="job-posted-by">
    <a class="gtmJobListingPostedBy">Tech Ltd</a>
  </div>
  <li data-qa="job-metadata-location">Remote</li>
</div>
</body></html>
`

func TestReedParser(t *testing.T) {
	p, ok := GetParser("reed")
	if !ok {
		t.Fatal("reed parser not registered")
	}

	jobs, err := p.ParseListings(testReedHTML, "https://www.reed.co.uk/jobs")
	if err != nil {
		t.Fatalf("ParseListings error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("got %d jobs, want 2", len(jobs))
	}

	// Job 1
	if jobs[0].Title != "Senior Go Developer" {
		t.Errorf("job[0].Title = %q", jobs[0].Title)
	}
	if jobs[0].Company != "Acme Corp" {
		t.Errorf("job[0].Company = %q", jobs[0].Company)
	}
	if jobs[0].Location != "London, UK" {
		t.Errorf("job[0].Location = %q", jobs[0].Location)
	}
	if jobs[0].Salary != "£80,000 - £100,000" {
		t.Errorf("job[0].Salary = %q", jobs[0].Salary)
	}
	if jobs[0].Source != "Reed" {
		t.Errorf("job[0].Source = %q", jobs[0].Source)
	}
	if jobs[0].Link != "https://www.reed.co.uk/jobs/senior-go-developer/12345" {
		t.Errorf("job[0].Link = %q", jobs[0].Link)
	}

	// Job 2
	if jobs[1].Title != "Python Developer" {
		t.Errorf("job[1].Title = %q", jobs[1].Title)
	}
	if jobs[1].Location != "Remote" {
		t.Errorf("job[1].Location = %q", jobs[1].Location)
	}
}

func TestReedAliases(t *testing.T) {
	for _, alias := range []string{"totaljobs", "cv-library", "cvlibrary"} {
		p, ok := GetParser(alias)
		if !ok {
			t.Errorf("alias %q not found", alias)
			continue
		}
		if p.Name() != "Reed" {
			t.Errorf("alias %q resolved to %q, want Reed", alias, p.Name())
		}
	}
}

func TestAllMajorParsersRegistered(t *testing.T) {
	for _, name := range []string{"reed", "linkedin", "indeed", "glassdoor", "adzuna", "technojobs"} {
		if _, ok := GetParser(name); !ok {
			t.Errorf("parser %q not registered", name)
		}
	}
}

const testReedDetailHTML = `
<html><body>
<div data-qa="job-description">
  We are looking for a Python developer with Django and React experience.

  Responsibilities
  You will build scalable web applications and design REST APIs.

  This is a remote position with hybrid options available.
</div>
<div class="benefits-list">
  Remote working, Flexible hours, Pension
</div>
</body></html>
`

func TestReedDetailParser(t *testing.T) {
	p, _ := GetParser("reed")
	dp, ok := p.(DetailParser)
	if !ok {
		t.Fatal("reed parser does not implement DetailParser")
	}

	details, err := dp.ParseDetails(testReedDetailHTML)
	if err != nil {
		t.Fatalf("ParseDetails error: %v", err)
	}
	if details["description"] == "" {
		t.Error("expected non-empty description")
	}
	if details["skills"] == "" {
		t.Error("expected non-empty skills")
	}
	if details["work_type"] == "" {
		t.Error("expected non-empty work_type")
	}
}
