package parsers

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/campbell/huntr-ai/internal/models"
)

func init() {
	Register("cwjobs", &cwJobsParser{})
	RegisterAlias("cw jobs", "cwjobs")
}

type cwJobsParser struct{}

func (p *cwJobsParser) Name() string { return "CWJobs" }

func (p *cwJobsParser) ParseListings(html string, sourceURL string) ([]models.Job, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("scraper: cwjobs parse: %w", err)
	}

	cards := FindByClassContaining(doc.Selection, "article", "job")
	if cards.Length() == 0 {
		cards = doc.Find("div.job-result")
	}
	if cards.Length() == 0 {
		cards = FindByClassContaining(doc.Selection, "div", "job")
	}

	slog.Info("found job cards", "source", "CWJobs", "count", cards.Length())

	var jobs []models.Job
	cards.EachWithBreak(func(_ int, card *goquery.Selection) bool {
		if len(jobs) >= 20 {
			return false
		}

		titleSel := card.Find("h2.job-title").First()
		if titleSel.Length() == 0 {
			titleSel = card.Find("a.job-title").First()
		}
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

		// Find link
		var linkSel *goquery.Selection
		if titleSel.Is("a") {
			linkSel = titleSel
		} else {
			linkSel = titleSel.Find("a[href]").First()
			if linkSel.Length() == 0 {
				linkSel = card.Find("a[href]").First()
			}
		}

		href := ""
		if linkSel != nil && linkSel.Length() > 0 {
			href, _ = linkSel.Attr("href")
		}

		company := ExtractText(card.Find(".company").First())
		if company == "" {
			company = ExtractText(FindByClassContaining(card, "*", "company").First())
		}
		if company == "" {
			company = "Unknown"
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

		if title != "" {
			jobs = append(jobs, models.Job{
				Title:    title,
				Company:  company,
				Location: location,
				Salary:   salary,
				Link:     ResolveURL(sourceURL, href),
				Source:   "CWJobs",
			})
		}
		return true
	})

	slog.Info("parsed jobs", "source", "CWJobs", "count", len(jobs))
	return jobs, nil
}

func (p *cwJobsParser) ParseDetails(html string) (map[string]string, error) {
	return ParseGenericDetails(html)
}
