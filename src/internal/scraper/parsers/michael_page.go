package parsers

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/campbell/huntr-ai/internal/models"
)

func init() {
	Register("michaelpage", &michaelPageParser{})
	RegisterAlias("michael page", "michaelpage")
}

type michaelPageParser struct{}

func (p *michaelPageParser) Name() string { return "Michael Page" }

func (p *michaelPageParser) ParseListings(html string, sourceURL string) ([]models.Job, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("scraper: michael page parse: %w", err)
	}

	cards := FindByClassContaining(doc.Selection, "article", "job")
	if cards.Length() == 0 {
		cards = FindByClassContaining(doc.Selection, "div", "result")
	}
	if cards.Length() == 0 {
		cards = FindByClassContaining(doc.Selection, "div", "job")
	}

	slog.Info("found job cards", "source", "Michael Page", "count", cards.Length())

	var jobs []models.Job
	cards.EachWithBreak(func(_ int, card *goquery.Selection) bool {
		if len(jobs) >= 20 {
			return false
		}

		titleSel := card.Find("h2").First()
		if titleSel.Length() == 0 {
			titleSel = card.Find("h3").First()
		}
		if titleSel.Length() == 0 {
			titleSel = card.Find("a[href]").First()
		}
		if titleSel.Length() == 0 {
			return true
		}

		title := ExtractText(titleSel)

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
			company = "Michael Page"
		}

		location := ExtractText(FindByClassContaining(card, "*", "location").First())
		if location == "" {
			location = "UK"
		}

		salary := ExtractText(FindByClassContaining(card, "*", "salary").First())

		if title != "" {
			jobs = append(jobs, models.Job{
				Title:    title,
				Company:  company,
				Location: location,
				Salary:   salary,
				Link:     ResolveURL(sourceURL, href),
				Source:   "Michael Page",
			})
		}
		return true
	})

	slog.Info("parsed jobs", "source", "Michael Page", "count", len(jobs))
	return jobs, nil
}

func (p *michaelPageParser) ParseDetails(html string) (map[string]string, error) {
	return ParseGenericDetails(html)
}
