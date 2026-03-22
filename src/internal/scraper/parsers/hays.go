package parsers

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/campbell/huntr-ai/internal/models"
)

func init() {
	Register("hays", &haysParser{})
}

type haysParser struct{}

func (p *haysParser) Name() string { return "Hays" }

func (p *haysParser) ParseListings(html string, sourceURL string) ([]models.Job, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("scraper: hays parse: %w", err)
	}

	// Try multiple selectors
	cards := FindByClassContaining(doc.Selection, "article", "job")
	if cards.Length() == 0 {
		cards = FindByClassContaining(doc.Selection, "div", "job")
	}
	if cards.Length() == 0 {
		cards = FindByClassContaining(doc.Selection, "li", "job")
	}

	slog.Info("found job cards", "source", "Hays", "count", cards.Length())

	var jobs []models.Job
	cards.EachWithBreak(func(_ int, card *goquery.Selection) bool {
		if len(jobs) >= 20 {
			return false
		}

		// Title: find first link or heading
		titleSel := card.Find("a[href]").First()
		if titleSel.Length() == 0 {
			titleSel = card.Find("h2").First()
		}
		if titleSel.Length() == 0 {
			titleSel = card.Find("h3").First()
		}
		if titleSel.Length() == 0 {
			return true
		}

		title := ExtractText(titleSel)
		if title == "" {
			return true
		}

		// Link
		var linkSel *goquery.Selection
		if titleSel.Is("a") {
			linkSel = titleSel
		} else {
			linkSel = card.Find("a[href]").First()
		}

		href := ""
		if linkSel != nil && linkSel.Length() > 0 {
			href, _ = linkSel.Attr("href")
		}

		company := ExtractText(FindByClassContaining(card, "*", "company").First())
		if company == "" {
			company = "Hays"
		}

		location := ExtractText(FindByClassContaining(card, "*", "location").First())
		if location == "" {
			location = "UK"
		}

		salary := ExtractText(FindByClassContaining(card, "*", "salary").First())

		jobs = append(jobs, models.Job{
			Title:    title,
			Company:  company,
			Location: location,
			Salary:   salary,
			Link:     ResolveURL(sourceURL, href),
			Source:   "Hays",
		})
		return true
	})

	slog.Info("parsed jobs", "source", "Hays", "count", len(jobs))
	return jobs, nil
}

func (p *haysParser) ParseDetails(html string) (map[string]string, error) {
	return ParseGenericDetails(html)
}
