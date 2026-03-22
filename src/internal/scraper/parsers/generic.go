package parsers

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/campbell/huntr-ai/internal/models"
)

func init() {
	Register("generic", &genericParser{})
}

// genericParser is the fallback parser for unknown sources.
type genericParser struct{}

func (p *genericParser) Name() string { return "Generic" }

func (p *genericParser) ParseListings(html string, sourceURL string) ([]models.Job, error) {
	return parseGenericListings(html, sourceURL, "Generic")
}

func (p *genericParser) ParseDetails(html string) (map[string]string, error) {
	return ParseGenericDetails(html)
}

// parseGenericListings tries multiple common selectors to extract job listings.
// Used directly by the generic parser and as a base for recruitment agency parsers.
func parseGenericListings(html string, sourceURL string, sourceName string) ([]models.Job, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("scraper: %s parse: %w", sourceName, err)
	}

	var cards *goquery.Selection

	// Try multiple selectors in priority order
	selectors := []string{
		"article",
		"div[class*='job']", "div[class*='Job']",
		"li[class*='job']", "li[class*='Job']",
		"div[class*='result']", "div[class*='Result']",
	}

	for _, sel := range selectors {
		found := doc.Find(sel)
		if found.Length() > 0 {
			cards = found
			break
		}
	}

	if cards == nil || cards.Length() == 0 {
		slog.Info("no job cards found", "source", sourceName)
		return nil, nil
	}

	slog.Info("found job cards", "source", sourceName, "count", cards.Length())

	var jobs []models.Job
	cards.Each(func(_ int, card *goquery.Selection) {
		// Find title: h2, h3, or first link with title-like class
		titleSel := card.Find("h2").First()
		if titleSel.Length() == 0 {
			titleSel = card.Find("h3").First()
		}
		if titleSel.Length() == 0 {
			titleSel = FindByClassContaining(card, "a", "title").First()
		}
		if titleSel.Length() == 0 {
			titleSel = card.Find("a[href]").First()
		}
		if titleSel.Length() == 0 {
			return
		}

		title := ExtractText(titleSel)
		if title == "" {
			return
		}

		// Find link
		var href string
		if titleSel.Is("a") {
			href, _ = titleSel.Attr("href")
		}
		if href == "" {
			linkSel := titleSel.Find("a[href]").First()
			if linkSel.Length() == 0 {
				linkSel = card.Find("a[href]").First()
			}
			if linkSel.Length() > 0 {
				href, _ = linkSel.Attr("href")
			}
		}
		if href == "" {
			return
		}

		// Extract optional fields with flexible selectors
		company := ExtractText(FindByClassContaining(card, "*", "company").First())
		if company == "" {
			company = ExtractText(FindByClassContaining(card, "span", "employer").First())
		}
		if company == "" {
			company = sourceName
		}

		location := ExtractText(FindByClassContaining(card, "*", "location").First())
		if location == "" {
			location = ExtractText(FindByClassContaining(card, "span", "place").First())
		}
		if location == "" {
			location = "UK"
		}

		salary := ExtractText(FindByClassContaining(card, "*", "salary").First())
		if salary == "" {
			salary = ExtractText(FindByClassContaining(card, "*", "pay").First())
		}

		jobs = append(jobs, models.Job{
			Title:    title,
			Company:  company,
			Location: location,
			Salary:   salary,
			Link:     ResolveURL(sourceURL, href),
			Source:   sourceName,
		})
	})

	slog.Info("parsed jobs", "source", sourceName, "count", len(jobs))
	return jobs, nil
}

// ParseGenericDetails is the fallback detail parser for unknown sources.
func ParseGenericDetails(html string) (map[string]string, error) {
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

	// Try common description selectors
	var descSel *goquery.Selection
	for _, sel := range []string{
		"div[class*='description']", "div[id*='description']",
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
