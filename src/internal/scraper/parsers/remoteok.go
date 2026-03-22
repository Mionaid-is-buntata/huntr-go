package parsers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/campbell/huntr-ai/internal/models"
)

func init() {
	Register("remoteok", &remoteOKParser{})
	RegisterAlias("remote ok", "remoteok")
}

type remoteOKParser struct{}

func (p *remoteOKParser) Name() string { return "Remote OK" }

type remoteOKJob struct {
	Position  string   `json:"position"`
	Company   string   `json:"company"`
	Location  string   `json:"location"`
	SalaryMin int      `json:"salary_min"`
	SalaryMax int      `json:"salary_max"`
	URL       string   `json:"url"`
	Slug      string   `json:"slug"`
	Desc      string   `json:"description"`
	Tags      []string `json:"tags"`
}

func (p *remoteOKParser) ParseListings(html string, sourceURL string) ([]models.Job, error) {
	// Try JSON API first
	var data []json.RawMessage
	if err := json.Unmarshal([]byte(html), &data); err == nil {
		return p.parseJSON(data)
	}

	// Fallback to HTML
	return p.parseHTML(html)
}

func (p *remoteOKParser) parseJSON(data []json.RawMessage) ([]models.Job, error) {
	var jobs []models.Job

	// First element is legal notice, skip it
	start := 0
	if len(data) > 1 {
		start = 1
	}

	limit := len(data)
	if limit > start+20 {
		limit = start + 20
	}

	for i := start; i < limit; i++ {
		var entry remoteOKJob
		if err := json.Unmarshal(data[i], &entry); err != nil {
			continue
		}

		salary := ""
		if entry.SalaryMin > 0 && entry.SalaryMax > 0 {
			salary = fmt.Sprintf("$%d - $%d", entry.SalaryMin, entry.SalaryMax)
		} else if entry.SalaryMin > 0 {
			salary = fmt.Sprintf("$%d+", entry.SalaryMin)
		}

		link := entry.URL
		if link == "" && entry.Slug != "" {
			link = "https://remoteok.com/" + entry.Slug
		}

		location := entry.Location
		if location == "" {
			location = "Remote"
		}

		desc := entry.Desc
		if len(desc) > 500 {
			desc = desc[:500]
		}

		jobs = append(jobs, models.Job{
			Title:       entry.Position,
			Company:     entry.Company,
			Location:    location,
			Salary:      salary,
			Link:        link,
			Source:      "Remote OK",
			Description: desc,
			Skills:      strings.Join(entry.Tags, ", "),
			WorkType:    "Remote",
		})
	}

	slog.Info("parsed jobs", "source", "Remote OK", "count", len(jobs))
	return jobs, nil
}

func (p *remoteOKParser) parseHTML(html string) ([]models.Job, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("scraper: remoteok parse: %w", err)
	}

	var jobs []models.Job
	doc.Find("tr.job").EachWithBreak(func(_ int, row *goquery.Selection) bool {
		if len(jobs) >= 20 {
			return false
		}
		titleSel := row.Find("h2").First()
		if titleSel.Length() == 0 {
			return true
		}

		title := ExtractText(titleSel)
		company := ExtractText(row.Find("h3").First())
		if company == "" {
			company = "Unknown"
		}

		link := ""
		if linkSel := row.Find("a.preventLink").First(); linkSel.Length() > 0 {
			if href, exists := linkSel.Attr("href"); exists {
				link = "https://remoteok.com" + href
			}
		}

		jobs = append(jobs, models.Job{
			Title:    title,
			Company:  company,
			Location: "Remote",
			Link:     link,
			Source:   "Remote OK",
			WorkType: "Remote",
		})
		return true
	})

	slog.Info("parsed jobs", "source", "Remote OK", "count", len(jobs))
	return jobs, nil
}

func (p *remoteOKParser) ParseDetails(html string) (map[string]string, error) {
	details := map[string]string{
		"description":      "",
		"skills":           "",
		"responsibilities": "",
		"benefits":         "",
		"work_type":        "Remote",
	}

	// Try JSON first
	var data []json.RawMessage
	if err := json.Unmarshal([]byte(html), &data); err == nil {
		idx := 0
		if len(data) > 1 {
			idx = 1
		}
		if idx < len(data) {
			var entry remoteOKJob
			if err := json.Unmarshal(data[idx], &entry); err == nil {
				details["description"] = entry.Desc
				details["skills"] = strings.Join(entry.Tags, ", ")
				return details, nil
			}
		}
	}

	// HTML fallback
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return details, nil
	}

	descSel := doc.Find(".description").First()
	if descSel.Length() == 0 {
		descSel = doc.Find(".markdown").First()
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

	return details, nil
}
