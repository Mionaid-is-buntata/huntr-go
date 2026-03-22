package parsers

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/campbell/huntr-ai/internal/models"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func init() {
	Register("otta", &ottaParser{})
	RegisterAlias("welcome to the jungle", "otta")
	RegisterAlias("wttj", "otta")
}

type ottaParser struct{}

func (p *ottaParser) Name() string { return "Otta" }

var (
	ottaJobLinkRe    = regexp.MustCompile(`/companies/([^/]+)/jobs/`)
	ottaAriaLabelRe  = regexp.MustCompile(`Visit the job post for (.+)`)
	ottaLocationRe   = regexp.MustCompile(`/jobs/[^_]+_([a-z-]+)(?:_|$)`)
)

func (p *ottaParser) ParseListings(html string, sourceURL string) ([]models.Job, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("scraper: otta parse: %w", err)
	}

	seen := make(map[string]bool)
	var jobs []models.Job

	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		if !strings.Contains(href, "/companies/") || !strings.Contains(href, "/jobs/") {
			return
		}
		if seen[href] {
			return
		}
		seen[href] = true

		// Title from aria-label or text
		title := ""
		if aria, exists := s.Attr("aria-label"); exists {
			if m := ottaAriaLabelRe.FindStringSubmatch(aria); len(m) > 1 {
				title = m[1]
			}
		}
		if title == "" {
			title = strings.TrimSpace(s.Text())
		}
		if title == "" {
			return
		}

		// Company from URL
		company := "Unknown"
		if m := ottaJobLinkRe.FindStringSubmatch(href); len(m) > 1 {
			company = cases.Title(language.English).String(strings.ReplaceAll(m[1], "-", " "))
		}

		// Location from URL suffix
		location := ""
		if m := ottaLocationRe.FindStringSubmatch(href); len(m) > 1 {
			location = cases.Title(language.English).String(strings.ReplaceAll(m[1], "-", " "))
		}

		// Absolute URL
		link := href
		if strings.HasPrefix(href, "/") {
			link = "https://www.welcometothejungle.com" + href
		}

		jobs = append(jobs, models.Job{
			Title:    title,
			Company:  company,
			Location: location,
			Link:     link,
			Source:   "Otta",
		})
	})

	slog.Info("parsed jobs", "source", "Otta", "count", len(jobs))
	return jobs, nil
}

func (p *ottaParser) ParseDetails(html string) (map[string]string, error) {
	details := map[string]string{
		"description":      "",
		"skills":           "",
		"responsibilities": "",
		"benefits":         "",
		"work_type":        "",
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return details, nil
	}

	// WTTJ detail pages use various content containers
	var descSel *goquery.Selection
	for _, sel := range []string{
		"div[class*='description']", "div[class*='content']",
		"article", "main",
	} {
		descSel = doc.Find(sel).First()
		if descSel.Length() > 0 {
			break
		}
	}

	if descSel != nil && descSel.Length() > 0 {
		desc := strings.TrimSpace(descSel.Text())
		if len(desc) > 2000 {
			desc = desc[:2000]
		}
		details["description"] = desc
	}

	if details["description"] != "" {
		details["skills"] = ExtractSkillsFromText(details["description"])
		details["responsibilities"] = ExtractResponsibilitiesFromText(details["description"])
	}

	details["work_type"] = DetectWorkType(doc.Text())

	return details, nil
}
