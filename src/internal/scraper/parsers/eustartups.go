package parsers

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/campbell/huntr-ai/internal/models"
)

func init() {
	Register("eustartups", &euStartupsParser{})
	RegisterAlias("eu-startups", "eustartups")
	RegisterAlias("eu startups", "eustartups")
}

type euStartupsParser struct{}

func (p *euStartupsParser) Name() string { return "EU-Startups" }

func (p *euStartupsParser) ParseListings(html string, sourceURL string) ([]models.Job, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("scraper: eustartups parse: %w", err)
	}

	rows := doc.Find(".wpjb-grid-row")

	var jobs []models.Job
	rows.EachWithBreak(func(_ int, row *goquery.Selection) bool {
		if len(jobs) >= 20 {
			return false
		}

		titleSel := row.Find(".wpjb-col-title .wpjb-line-major a").First()
		if titleSel.Length() == 0 {
			titleSel = row.Find(".wpjb-col-title a").First()
		}
		if titleSel.Length() == 0 {
			return true
		}

		title := ExtractText(titleSel)
		link, _ := titleSel.Attr("href")

		// Location
		location := ExtractText(row.Find(".wpjb-col-location .wpjb-line-major").First())
		if location == "" {
			location = "Europe"
		}

		// Company from logo alt
		company := ""
		if logo := row.Find(".wpjb-col-logo img").First(); logo.Length() > 0 {
			if alt, exists := logo.Attr("alt"); exists {
				company = strings.TrimSuffix(strings.TrimSpace(alt), " logo")
			}
		}

		// Work type from classes
		workType := "Full-time"
		rowClasses, _ := row.Attr("class")
		rowClasses = strings.ToLower(rowClasses)
		titleLower := strings.ToLower(title)
		if strings.Contains(rowClasses, "remote") || strings.Contains(titleLower, "remote") {
			workType = "Remote"
		} else if strings.Contains(rowClasses, "part-time") {
			workType = "Part-time"
		}

		if title != "" {
			jobs = append(jobs, models.Job{
				Title:    title,
				Company:  company,
				Location: location,
				Link:     link,
				Source:   "EU-Startups",
				WorkType: workType,
			})
		}
		return true
	})

	slog.Info("parsed jobs", "source", "EU-Startups", "count", len(jobs))
	return jobs, nil
}

func (p *euStartupsParser) ParseDetails(html string) (map[string]string, error) {
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

	var descSel *goquery.Selection
	for _, sel := range []string{".wpjb-job-content", ".job-description", "article"} {
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
