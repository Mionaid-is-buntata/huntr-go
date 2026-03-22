package parsers

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/campbell/huntr-ai/internal/models"
)

func init() {
	Register("jobsite", &jobsiteParser{})
}

type jobsiteParser struct{}

func (p *jobsiteParser) Name() string { return "Jobsite" }

func (p *jobsiteParser) ParseListings(html string, sourceURL string) ([]models.Job, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("scraper: jobsite parse: %w", err)
	}

	// Try multiple selectors
	cards := doc.Find("article[data-job-id]")
	if cards.Length() == 0 {
		cards = doc.Find(".job-card")
	}
	if cards.Length() == 0 {
		cards = FindByClassContaining(doc.Selection, "div", "job-result")
	}
	if cards.Length() == 0 {
		cards = doc.Find(".search-result")
	}

	var jobs []models.Job
	cards.EachWithBreak(func(_ int, card *goquery.Selection) bool {
		if len(jobs) >= 20 {
			return false
		}

		titleSel := card.Find("h2 a").First()
		if titleSel.Length() == 0 {
			titleSel = card.Find(".job-title a").First()
		}
		if titleSel.Length() == 0 {
			titleSel = FindByClassContaining(card, "a", "title").First()
		}
		if titleSel.Length() == 0 {
			return true
		}

		title := ExtractText(titleSel)
		href, _ := titleSel.Attr("href")
		link := ResolveURL("https://www.jobsite.co.uk", href)

		company := ExtractText(card.Find(".company-name").First())
		if company == "" {
			company = ExtractText(FindByClassContaining(card, "*", "company").First())
		}

		location := ExtractText(card.Find(".location").First())
		if location == "" {
			location = ExtractText(FindByClassContaining(card, "*", "location").First())
		}

		salary := ExtractText(card.Find(".salary").First())
		if salary == "" {
			salary = ExtractText(FindByClassContaining(card, "*", "salary").First())
		}

		if title != "" {
			jobs = append(jobs, models.Job{
				Title:    title,
				Company:  company,
				Location: location,
				Salary:   salary,
				Link:     link,
				Source:   "Jobsite",
				WorkType: "Full-time",
			})
		}
		return true
	})

	slog.Info("parsed jobs", "source", "Jobsite", "count", len(jobs))
	return jobs, nil
}

// Jobsite detail pages use the same layout as Totaljobs (same company).
func (p *jobsiteParser) ParseDetails(html string) (map[string]string, error) {
	return ParseGenericDetails(html)
}
