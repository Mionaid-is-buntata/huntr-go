package parsers

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/campbell/huntr-ai/internal/models"
)

func init() {
	Register("indeed", &indeedParser{})
	RegisterAlias("indeed uk", "indeed")
}

type indeedParser struct{}

func (p *indeedParser) Name() string { return "Indeed" }

func (p *indeedParser) ParseListings(html string, sourceURL string) ([]models.Job, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("scraper: indeed parse: %w", err)
	}

	cards := doc.Find("div[data-jk]")
	if cards.Length() == 0 {
		cards = doc.Find("div.job_seen_beacon")
	}
	if cards.Length() == 0 {
		cards = doc.Find("td.resultContent")
	}

	slog.Info("found job cards", "source", "Indeed", "count", cards.Length())

	var jobs []models.Job
	cards.Each(func(_ int, card *goquery.Selection) {
		titleSel := card.Find("h2.jobTitle").First()
		if titleSel.Length() == 0 {
			titleSel = card.Find("a[data-jk]").First()
		}
		if titleSel.Length() == 0 {
			return
		}

		title := ExtractText(titleSel)
		if title == "" {
			return
		}

		company := ExtractText(card.Find("span.companyName").First())
		if company == "" {
			company = ExtractText(card.Find("a[data-testid='company-name']").First())
		}
		if company == "" {
			company = "Unknown"
		}

		location := ExtractText(card.Find("div.companyLocation").First())
		if location == "" {
			location = ExtractText(card.Find("div[data-testid='text-location']").First())
		}
		if location == "" {
			location = "Unknown"
		}

		salary := ExtractText(card.Find("span.salaryText").First())
		if salary == "" {
			salary = ExtractText(card.Find("span[data-testid='attribute_snippet_testid']").First())
		}

		href, _ := card.Find("a[href]").First().Attr("href")
		link := ResolveURL("https://uk.indeed.com", href)
		if link == "" {
			link = sourceURL
		}

		jobs = append(jobs, models.Job{
			Title:    title,
			Company:  company,
			Location: location,
			Salary:   salary,
			Link:     link,
			Source:   "Indeed",
		})
	})

	slog.Info("parsed jobs", "source", "Indeed", "count", len(jobs))
	return jobs, nil
}

func (p *indeedParser) ParseDetails(html string) (map[string]string, error) {
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

	descSel := doc.Find("div#jobDescriptionText").First()
	if descSel.Length() == 0 {
		descSel = doc.Find("div.jobsearch-jobDescriptionText").First()
	}
	if descSel.Length() == 0 {
		descSel = doc.Find("div[data-testid='jobDescriptionText']").First()
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
