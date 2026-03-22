package parsers

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/campbell/huntr-ai/internal/models"
)

func init() {
	Register("roberthalf", &robertHalfParser{})
	RegisterAlias("robert half", "roberthalf")
}

type robertHalfParser struct{}

func (p *robertHalfParser) Name() string { return "Robert Half" }

func (p *robertHalfParser) ParseListings(html string, sourceURL string) ([]models.Job, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("scraper: robert half parse: %w", err)
	}

	// Try multiple selectors
	cards := doc.Find(".job-card")
	if cards.Length() == 0 {
		cards = doc.Find(".job-listing")
	}
	if cards.Length() == 0 {
		cards = FindByClassContaining(doc.Selection, "div", "job-result")
	}
	if cards.Length() == 0 {
		cards = doc.Find("article.job")
	}
	if cards.Length() == 0 {
		cards = doc.Find(".rh-search-result")
	}

	slog.Info("found job cards", "source", "Robert Half", "count", cards.Length())

	var jobs []models.Job
	cards.EachWithBreak(func(_ int, card *goquery.Selection) bool {
		if len(jobs) >= 20 {
			return false
		}

		titleSel := card.Find("h2 a").First()
		if titleSel.Length() == 0 {
			titleSel = card.Find("h3 a").First()
		}
		if titleSel.Length() == 0 {
			titleSel = card.Find(".job-title a").First()
		}
		if titleSel.Length() == 0 {
			titleSel = card.Find("a.title").First()
		}
		if titleSel.Length() == 0 {
			return true
		}

		title := ExtractText(titleSel)
		href, _ := titleSel.Attr("href")
		link := ResolveURL("https://www.roberthalf.com", href)

		company := ExtractText(card.Find(".company").First())
		if company == "" {
			company = ExtractText(FindByClassContaining(card, "*", "company").First())
		}
		if company == "" {
			company = "Robert Half"
		}

		location := ExtractText(card.Find(".location").First())
		if location == "" {
			location = ExtractText(FindByClassContaining(card, "*", "location").First())
		}
		if location == "" {
			location = "UK"
		}

		salary := ExtractText(card.Find(".salary").First())
		if salary == "" {
			salary = ExtractText(FindByClassContaining(card, "*", "salary").First())
		}
		if salary == "" {
			salary = ExtractText(FindByClassContaining(card, "*", "compensation").First())
		}

		// Work type
		workType := "Full-time"
		itemText := strings.ToLower(card.Text())
		if strings.Contains(itemText, "remote") {
			workType = "Remote"
			if strings.Contains(itemText, "hybrid") {
				workType = "Hybrid"
			}
		} else if strings.Contains(itemText, "hybrid") {
			workType = "Hybrid"
		} else if strings.Contains(itemText, "contract") {
			workType = "Contract"
		}

		if title != "" {
			jobs = append(jobs, models.Job{
				Title:    title,
				Company:  company,
				Location: location,
				Salary:   salary,
				Link:     link,
				Source:   "Robert Half",
				WorkType: workType,
			})
		}
		return true
	})

	slog.Info("parsed jobs", "source", "Robert Half", "count", len(jobs))
	return jobs, nil
}

func (p *robertHalfParser) ParseDetails(html string) (map[string]string, error) {
	return ParseGenericDetails(html)
}
