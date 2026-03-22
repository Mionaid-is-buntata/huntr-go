package parsers

import (
	"testing"
)

func TestGenericParserRegistered(t *testing.T) {
	p, ok := GetParser("generic")
	if !ok {
		t.Fatal("generic parser not registered")
	}
	if p.Name() != "Generic" {
		t.Errorf("generic parser name = %q, want Generic", p.Name())
	}
}

const testGenericHTML = `
<html><body>
<article>
  <h2><a href="/jobs/123">Senior Go Developer</a></h2>
  <span class="company-name">Acme Corp</span>
  <span class="location">London, UK</span>
  <span class="salary">£80,000 - £100,000</span>
</article>
<article>
  <h2><a href="/jobs/456">Python Engineer</a></h2>
  <span class="company-name">Tech Ltd</span>
  <span class="location">Remote</span>
</article>
<article>
  <h3>No Link Job</h3>
</article>
</body></html>
`

func TestGenericParserParseListings(t *testing.T) {
	p, _ := GetParser("generic")
	jobs, err := p.ParseListings(testGenericHTML, "https://example.com")
	if err != nil {
		t.Fatalf("ParseListings error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("got %d jobs, want 2", len(jobs))
	}

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
	if jobs[0].Link != "https://example.com/jobs/123" {
		t.Errorf("job[0].Link = %q", jobs[0].Link)
	}

	if jobs[1].Title != "Python Engineer" {
		t.Errorf("job[1].Title = %q", jobs[1].Title)
	}
	if jobs[1].Location != "Remote" {
		t.Errorf("job[1].Location = %q", jobs[1].Location)
	}
}

const testGenericDetailHTML = `
<html><body>
<div class="job-description">
  We are looking for a Python developer with experience in Django and React.

  Responsibilities
  You will build scalable web applications.
  You will design REST APIs.

  Benefits include remote working and flexible hours.
</div>
</body></html>
`

func TestGenericParseDetails(t *testing.T) {
	details, err := ParseGenericDetails(testGenericDetailHTML)
	if err != nil {
		t.Fatalf("ParseGenericDetails error: %v", err)
	}
	if details["description"] == "" {
		t.Error("expected non-empty description")
	}
	if details["skills"] == "" {
		t.Error("expected non-empty skills")
	}
}

func TestGetParserAlias(t *testing.T) {
	// Register a test alias
	RegisterAlias("testsite", "generic")

	p, ok := GetParser("testsite")
	if !ok {
		t.Fatal("alias lookup failed")
	}
	if p.Name() != "Generic" {
		t.Errorf("alias resolved to %q, want Generic", p.Name())
	}
}

func TestGetParserPartialMatch(t *testing.T) {
	// "generic" is registered; "some generic site" should partially match
	p, ok := GetParser("some generic site")
	if !ok {
		t.Fatal("partial match failed")
	}
	if p.Name() != "Generic" {
		t.Errorf("partial match resolved to %q, want Generic", p.Name())
	}
}

func TestGetParserNotFound(t *testing.T) {
	_, ok := GetParser("nonexistentsitexyz123")
	if ok {
		t.Error("expected no parser found for nonexistent site")
	}
}
