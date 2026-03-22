package parsers

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/campbell/huntr-ai/internal/models"
)

func init() {
	Register("technojobs", &technojobsParser{})
}

type technojobsParser struct{}

func (p *technojobsParser) Name() string { return "Technojobs" }

func (p *technojobsParser) ParseListings(html string, sourceURL string) ([]models.Job, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("scraper: technojobs parse: %w", err)
	}

	cards := doc.Find("div.job-listing")
	if cards.Length() == 0 {
		cards = doc.Find("article.job")
	}
	if cards.Length() == 0 {
		cards = doc.Find("div.job-item")
	}
	if cards.Length() == 0 {
		cards = doc.Find("li.job-result")
	}

	slog.Info("found job cards", "source", "Technojobs", "count", cards.Length())

	var jobs []models.Job
	cards.Each(func(_ int, card *goquery.Selection) {
		titleSel := card.Find("a.job-title").First()
		if titleSel.Length() == 0 {
			titleSel = card.Find("h2").First()
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

		company := ExtractText(card.Find("span.company").First())
		if company == "" {
			company = ExtractText(card.Find("div.company").First())
		}
		if company == "" {
			company = ExtractText(card.Find("a.company").First())
		}
		if company == "" {
			company = "Unknown"
		}

		location := ExtractText(card.Find("span.location").First())
		if location == "" {
			location = ExtractText(card.Find("div.location").First())
		}
		if location == "" {
			location = "Unknown"
		}

		salary := ExtractText(card.Find("span.salary").First())
		if salary == "" {
			salary = ExtractText(card.Find("div.salary").First())
		}

		href, _ := card.Find("a[href]").First().Attr("href")
		link := ResolveURL("https://www.technojobs.co.uk", href)
		if link == "" {
			link = sourceURL
		}

		jobs = append(jobs, models.Job{
			Title:    title,
			Company:  company,
			Location: location,
			Salary:   salary,
			Link:     link,
			Source:   "Technojobs",
		})
	})

	slog.Info("parsed jobs", "source", "Technojobs", "count", len(jobs))
	return jobs, nil
}

func (p *technojobsParser) ParseDetails(html string) (map[string]string, error) {
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

	descSel := doc.Find("div.job-description").First()
	if descSel.Length() == 0 {
		descSel = doc.Find("div#job-description").First()
	}
	if descSel.Length() == 0 {
		descSel = doc.Find("article.job-details").First()
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

	// Check for explicit skills list
	skillsSel := doc.Find("ul.skills").First()
	if skillsSel.Length() == 0 {
		skillsSel = doc.Find("div.skills-required").First()
	}
	if skillsSel.Length() > 0 {
		var skillsList []string
		skillsSel.Find("li").Each(func(_ int, li *goquery.Selection) {
			if s := ExtractText(li); s != "" {
				skillsList = append(skillsList, s)
			}
		})
		if len(skillsList) > 15 {
			skillsList = skillsList[:15]
		}
		if len(skillsList) > 0 {
			details["skills"] = strings.Join(skillsList, ", ")
		}
	}

	details["work_type"] = DetectWorkType(doc.Text())
	return details, nil
}
