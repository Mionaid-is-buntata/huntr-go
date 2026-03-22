package parsers

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/campbell/huntr-ai/internal/models"
)

func init() {
	Register("adzuna", &adzunaParser{})
}

type adzunaParser struct{}

func (p *adzunaParser) Name() string { return "Adzuna" }

func (p *adzunaParser) ParseListings(html string, sourceURL string) ([]models.Job, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("scraper: adzuna parse: %w", err)
	}

	cards := doc.Find("div.a-card")
	if cards.Length() == 0 {
		cards = doc.Find("article.job")
	}
	if cards.Length() == 0 {
		cards = doc.Find("article[data-aid]")
	}
	if cards.Length() == 0 {
		cards = doc.Find("div[data-aid]")
	}

	slog.Info("found job cards", "source", "Adzuna", "count", cards.Length())

	var jobs []models.Job
	cards.Each(func(_ int, card *goquery.Selection) {
		titleSel := card.Find("h2").First()
		if titleSel.Length() > 0 {
			// If h2 found, get the link inside it
			if link := titleSel.Find("a").First(); link.Length() > 0 {
				titleSel = link
			}
		}
		if titleSel.Length() == 0 {
			titleSel = card.Find("a.a-title").First()
		}
		if titleSel.Length() == 0 {
			titleSel = card.Find("a[data-aid='job-title']").First()
		}
		if titleSel.Length() == 0 {
			return
		}

		title := ExtractText(titleSel)
		if title == "" {
			return
		}

		company := ExtractText(card.Find("div.ui-company").First())
		if company == "" {
			company = ExtractText(card.Find("span.a-company").First())
		}
		if company == "" {
			company = ExtractText(card.Find("div.company").First())
		}
		if company == "" {
			company = "Unknown"
		}

		location := ExtractText(card.Find("div.ui-location").First())
		if location == "" {
			location = ExtractText(card.Find("span.a-location").First())
		}
		if location == "" {
			location = ExtractText(card.Find("div.a-location").First())
		}
		if location == "" {
			location = "Unknown"
		}

		salary := ExtractText(card.Find("div.ui-salary").First())
		if salary == "" {
			salary = ExtractText(card.Find("span.a-salary").First())
		}
		if salary == "" {
			salary = ExtractText(card.Find("div.salary").First())
		}

		href, _ := card.Find("a[href]").First().Attr("href")
		link := ResolveURL("https://www.adzuna.co.uk", href)
		if link == "" {
			link = sourceURL
		}

		jobs = append(jobs, models.Job{
			Title:    title,
			Company:  company,
			Location: location,
			Salary:   salary,
			Link:     link,
			Source:   "Adzuna",
		})
	})

	slog.Info("parsed jobs", "source", "Adzuna", "count", len(jobs))
	return jobs, nil
}

func (p *adzunaParser) ParseDetails(html string) (map[string]string, error) {
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

	descSel := doc.Find("div.adp-body").First()
	if descSel.Length() == 0 {
		descSel = doc.Find("section.adp-body").First()
	}
	if descSel.Length() == 0 {
		descSel = doc.Find("div[itemprop='description']").First()
	}

	if descSel.Length() > 0 {
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
