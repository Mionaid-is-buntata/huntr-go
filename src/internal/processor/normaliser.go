package processor

import (
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/campbell/huntr-ai/internal/models"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var (
	abbreviations = map[string]string{
		"Dev": "Developer", "Eng": "Engineer", "Sr": "Senior",
		"Jr": "Junior", "Mgr": "Manager", "Dir": "Director",
	}

	salaryKRegex     = regexp.MustCompile(`(?i)(\d+\.?\d*)\s*k`)
	salaryNumRegex   = regexp.MustCompile(`(\d+)`)
	whitespaceRegex  = regexp.MustCompile(`\s+`)
	currencyStripper = regexp.MustCompile(`[£$€,\s]`)

	remotePatterns = []string{"remote", "work from home", "wfh", "home based"}
	ukCities       = map[string]string{
		"london": "London", "manchester": "Manchester", "birmingham": "Birmingham",
		"edinburgh": "Edinburgh", "glasgow": "Glasgow", "bristol": "Bristol",
		"leeds": "Leeds", "liverpool": "Liverpool",
	}
)

// NormaliseTitle converts a job title to standard format with expanded abbreviations.
func NormaliseTitle(title string) string {
	if title == "" {
		return ""
	}
	normalised := strings.TrimSpace(title)
	normalised = cases.Title(language.English).String(normalised)

	for abbrev, full := range abbreviations {
		re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(abbrev) + `\b`)
		normalised = re.ReplaceAllString(normalised, full)
	}
	return normalised
}

// ParseSalary extracts a numeric salary value from a string.
// Returns (original_string, numeric_value_in_GBP_or_nil).
func ParseSalary(salaryStr string) (string, *int) {
	if salaryStr == "" {
		return "", nil
	}
	original := strings.TrimSpace(salaryStr)
	cleaned := currencyStripper.ReplaceAllString(original, "")

	// Handle k/K suffix
	if m := salaryKRegex.FindStringSubmatch(cleaned); m != nil {
		f, _ := strconv.ParseFloat(m[1], 64)
		v := int(f * 1000)
		return original, &v
	}

	// Extract first number
	if m := salaryNumRegex.FindString(cleaned); m != "" {
		v, _ := strconv.Atoi(m)
		if v < 1000 {
			v *= 1000
		}
		return original, &v
	}

	return original, nil
}

// StandardiseLocation normalises location strings.
func StandardiseLocation(location string) string {
	if location == "" {
		return ""
	}
	location = strings.TrimSpace(location)
	lower := strings.ToLower(location)

	for _, p := range remotePatterns {
		if strings.Contains(lower, p) {
			if strings.Contains(location, "/") {
				parts := strings.Split(location, "/")
				for i, part := range parts {
					trimmed := strings.TrimSpace(part)
					for _, rp := range remotePatterns {
						if strings.EqualFold(trimmed, rp) {
							parts[i] = "Remote"
						}
					}
				}
				return strings.Join(parts, " / ")
			}
			return "Remote"
		}
	}

	for key, val := range ukCities {
		if strings.Contains(lower, key) {
			return val
		}
	}
	return location
}

// StandardiseWorkType returns a consistent work type value.
func StandardiseWorkType(workType, location string) string {
	if workType == "" && location == "" {
		return ""
	}
	combined := strings.ToLower(workType + " " + location)

	if strings.Contains(combined, "remote") || strings.Contains(combined, "work from home") || strings.Contains(combined, "wfh") {
		if strings.Contains(combined, "hybrid") {
			return "Hybrid"
		}
		return "Remote"
	}
	if strings.Contains(combined, "hybrid") {
		return "Hybrid"
	}
	if strings.Contains(combined, "on-site") || strings.Contains(combined, "onsite") || strings.Contains(combined, "office") {
		return "On-site"
	}
	return strings.TrimSpace(workType)
}

// CleanText removes extra whitespace and encoding issues.
func CleanText(text string) string {
	if text == "" {
		return ""
	}
	text = strings.TrimSpace(text)
	text = whitespaceRegex.ReplaceAllString(text, " ")
	text = strings.ReplaceAll(text, "\u00a0", " ")  // non-breaking space
	text = strings.ReplaceAll(text, "\u200b", "")    // zero-width space
	return text
}

// RemoveDuplicates removes duplicate jobs by (title|company|location) key.
func RemoveDuplicates(jobs []models.Job) []models.Job {
	seen := make(map[string]bool)
	var unique []models.Job
	for _, j := range jobs {
		key := fmt.Sprintf("%s|%s|%s",
			strings.ToLower(strings.TrimSpace(j.Title)),
			strings.ToLower(strings.TrimSpace(j.Company)),
			strings.ToLower(strings.TrimSpace(j.Location)),
		)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, j)
		}
	}
	removed := len(jobs) - len(unique)
	if removed > 0 {
		slog.Info("removed duplicate jobs", "count", removed)
	}
	return unique
}

// NormaliseJobs normalises a list of raw jobs.
func NormaliseJobs(rawJobs []models.Job) []models.Job {
	slog.Info("starting normalisation", "count", len(rawJobs))
	var normalised []models.Job

	for _, job := range rawJobs {
		location := StandardiseLocation(job.Location)
		workType := StandardiseWorkType(job.WorkType, location)

		desc := CleanText(job.Description)
		if len(desc) > 2000 {
			desc = desc[:2000]
		}

		_, salaryNum := ParseSalary(job.Salary)

		n := models.Job{
			Title:            NormaliseTitle(job.Title),
			Company:          strings.TrimSpace(job.Company),
			Location:         location,
			WorkType:         workType,
			Salary:           job.Salary,
			SalaryNum:        salaryNum,
			Description:      desc,
			Responsibilities: CleanText(job.Responsibilities),
			Skills:           CleanText(job.Skills),
			Benefits:         CleanText(job.Benefits),
			Link:             job.Link,
			Source:           job.Source,
		}
		if n.Source == "" {
			n.Source = "Unknown"
		}
		normalised = append(normalised, n)
	}

	normalised = RemoveDuplicates(normalised)
	slog.Info("normalisation complete", "count", len(normalised))
	return normalised
}
