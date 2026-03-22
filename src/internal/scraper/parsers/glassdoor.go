package parsers

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/campbell/huntr-ai/internal/models"
)

func init() {
	Register("glassdoor", &glassdoorParser{})
	RegisterAlias("glassdoor uk", "glassdoor")
}

type glassdoorParser struct{}

func (p *glassdoorParser) Name() string { return "Glassdoor" }

func (p *glassdoorParser) ParseListings(html string, sourceURL string) ([]models.Job, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("scraper: glassdoor parse: %w", err)
	}

	cards := doc.Find("li.react-job-listing")
	if cards.Length() == 0 {
		cards = doc.Find("div.jobContainer")
	}
	if cards.Length() == 0 {
		cards = doc.Find("article[data-test='job-listing']")
	}

	slog.Info("found job cards", "source", "Glassdoor", "count", cards.Length())

	var jobs []models.Job
	cards.Each(func(_ int, card *goquery.Selection) {
		titleSel := card.Find("a[data-test='job-link']").First()
		if titleSel.Length() == 0 {
			titleSel = card.Find("h2.jobTitle").First()
		}
		if titleSel.Length() == 0 {
			return
		}

		title := ExtractText(titleSel)
		if title == "" {
			return
		}

		company := ExtractText(card.Find("div.employerName").First())
		if company == "" {
			company = ExtractText(card.Find("span[data-test='employer-name']").First())
		}
		if company == "" {
			company = "Unknown"
		}

		location := ExtractText(card.Find("div.location").First())
		if location == "" {
			location = ExtractText(card.Find("span[data-test='job-location']").First())
		}
		if location == "" {
			location = "Unknown"
		}

		salary := ExtractText(card.Find("span.css-1hbqxpx").First())
		if salary == "" {
			salary = ExtractText(card.Find("div.salaryEstimate").First())
		}

		href, _ := card.Find("a[href]").First().Attr("href")
		link := ResolveURL("https://www.glassdoor.co.uk", href)
		if link == "" {
			link = sourceURL
		}

		jobs = append(jobs, models.Job{
			Title:    title,
			Company:  company,
			Location: location,
			Salary:   salary,
			Link:     link,
			Source:   "Glassdoor",
		})
	})

	slog.Info("parsed jobs", "source", "Glassdoor", "count", len(jobs))
	return jobs, nil
}

func (p *glassdoorParser) ParseDetails(html string) (map[string]string, error) {
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

	descSel := doc.Find("div.jobDescriptionContent").First()
	if descSel.Length() == 0 {
		descSel = doc.Find("div[data-test='job-description']").First()
	}
	if descSel.Length() == 0 {
		descSel = doc.Find("div.desc").First()
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
