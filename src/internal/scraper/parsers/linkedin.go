package parsers

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/campbell/huntr-ai/internal/models"
)

func init() {
	Register("linkedin", &linkedinParser{})
	RegisterAlias("linkedin jobs", "linkedin")
}

type linkedinParser struct{}

func (p *linkedinParser) Name() string { return "LinkedIn" }

func (p *linkedinParser) ParseListings(html string, sourceURL string) ([]models.Job, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("scraper: linkedin parse: %w", err)
	}

	cards := doc.Find("div.job-search-card")
	if cards.Length() == 0 {
		cards = doc.Find("div[data-job-id]")
	}
	if cards.Length() == 0 {
		cards = doc.Find("li.jobs-search-results__list-item")
	}

	slog.Info("found job cards", "source", "LinkedIn", "count", cards.Length())

	var jobs []models.Job
	cards.Each(func(_ int, card *goquery.Selection) {
		titleSel := card.Find("a.job-card-list__title").First()
		if titleSel.Length() == 0 {
			titleSel = card.Find("h3.base-search-card__title").First()
		}
		if titleSel.Length() == 0 {
			return
		}

		title := ExtractText(titleSel)
		if title == "" {
			return
		}

		company := ExtractText(card.Find("h4.base-search-card__subtitle").First())
		if company == "" {
			company = ExtractText(card.Find("a.job-card-container__company-name").First())
		}
		if company == "" {
			company = "Unknown"
		}

		location := ExtractText(card.Find("span.job-search-card__location").First())
		if location == "" {
			location = ExtractText(card.Find("span.job-search-card__location-info").First())
		}
		if location == "" {
			location = "Unknown"
		}

		href, _ := card.Find("a[href]").First().Attr("href")
		link := ResolveURL("https://www.linkedin.com", href)
		if link == "" {
			link = sourceURL
		}

		jobs = append(jobs, models.Job{
			Title:    title,
			Company:  company,
			Location: location,
			Link:     link,
			Source:   "LinkedIn",
		})
	})

	slog.Info("parsed jobs", "source", "LinkedIn", "count", len(jobs))
	return jobs, nil
}

func (p *linkedinParser) ParseDetails(html string) (map[string]string, error) {
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

	descSel := doc.Find("div.description__text").First()
	if descSel.Length() == 0 {
		descSel = doc.Find("div.show-more-less-html__markup").First()
	}
	if descSel.Length() == 0 {
		descSel = FindByClassContaining(doc.Selection, "div", "description").First()
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
