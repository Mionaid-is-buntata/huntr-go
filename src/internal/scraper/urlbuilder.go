package scraper

import (
	"fmt"
	"net/url"
	"strings"
)

// BuildSearchURL constructs a search URL with keywords for a given source.
// If no keywords are provided, returns the base URL unchanged.
func BuildSearchURL(baseURL string, sourceName string, keywords []string) string {
	if len(keywords) == 0 {
		return baseURL
	}

	src := strings.ToLower(sourceName)

	switch {
	case strings.Contains(src, "linkedin"):
		return buildLinkedInURL(baseURL, keywords)

	case strings.Contains(src, "indeed"):
		return buildIndeedURL(baseURL, keywords)

	case strings.Contains(src, "glassdoor"):
		kw := strings.ToLower(strings.Join(keywords[:min(2, len(keywords))], "-"))
		kw = strings.ReplaceAll(kw, " ", "-")
		return fmt.Sprintf("https://www.glassdoor.co.uk/Job/%s-jobs-SRCH_KO0,%d.htm", kw, len(kw))

	case strings.Contains(src, "reed"):
		kw := strings.ToLower(strings.Join(keywords[:min(2, len(keywords))], "-"))
		kw = strings.ReplaceAll(kw, " ", "-")
		return fmt.Sprintf("https://www.reed.co.uk/jobs/%s-jobs", kw)

	case strings.Contains(src, "adzuna"):
		kw := strings.Join(keywords[:min(3, len(keywords))], " ")
		return fmt.Sprintf("https://www.adzuna.co.uk/jobs/search?q=%s", url.QueryEscape(kw))

	case strings.Contains(src, "technojobs"):
		kw := strings.ToLower(strings.ReplaceAll(keywords[0], " ", "-"))
		return fmt.Sprintf("https://www.technojobs.co.uk/jobs/%s", kw)

	default:
		return baseURL
	}
}

func buildLinkedInURL(baseURL string, keywords []string) string {
	kw := keywords
	if len(kw) > 3 {
		kw = kw[:3]
	}

	if strings.Contains(baseURL, "search") {
		u, err := url.Parse(baseURL)
		if err != nil {
			return baseURL
		}
		q := u.Query()
		q.Set("keywords", strings.Join(kw, " "))
		if q.Get("location") == "" {
			q.Set("location", "United Kingdom")
		}
		u.RawQuery = q.Encode()
		return u.String()
	}

	kwStr := strings.Join(kw, " ")
	return fmt.Sprintf("https://www.linkedin.com/jobs/search/?keywords=%s&location=United%%20Kingdom", url.QueryEscape(kwStr))
}

func buildIndeedURL(baseURL string, keywords []string) string {
	kw := keywords
	if len(kw) > 3 {
		kw = kw[:3]
	}

	if strings.Contains(baseURL, "jobs") {
		u, err := url.Parse(baseURL)
		if err != nil {
			return baseURL
		}
		q := u.Query()
		q.Set("q", strings.Join(kw, " "))
		if q.Get("l") == "" {
			q.Set("l", "United Kingdom")
		}
		u.RawQuery = q.Encode()
		return u.String()
	}

	kwStr := strings.Join(kw, " ")
	return fmt.Sprintf("https://uk.indeed.com/jobs?q=%s&l=United+Kingdom", url.QueryEscape(kwStr))
}
