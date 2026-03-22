package parsers

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/campbell/huntr-ai/internal/models"
)

func init() {
	Register("weworkremotely", &weWorkRemotelyParser{})
	RegisterAlias("we work remotely", "weworkremotely")
	RegisterAlias("wwr", "weworkremotely")
}

type weWorkRemotelyParser struct{}

func (p *weWorkRemotelyParser) Name() string { return "We Work Remotely" }

func (p *weWorkRemotelyParser) ParseListings(html string, sourceURL string) ([]models.Job, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("scraper: weworkremotely parse: %w", err)
	}

	// Find job items
	items := doc.Find("li.feature, li.new-feature")
	if items.Length() == 0 {
		items = doc.Find("section.jobs li")
	}
	if items.Length() == 0 {
		items = doc.Find("article.job")
	}
	if items.Length() == 0 {
		items = doc.Find(".jobs-list li")
	}

	var jobs []models.Job
	items.EachWithBreak(func(_ int, item *goquery.Selection) bool {
		if len(jobs) >= 20 {
			return false
		}

		linkSel := item.Find("a[href*='/remote-jobs/']").First()
		if linkSel.Length() == 0 {
			return true
		}

		// Title
		titleSel := item.Find("span.title").First()
		if titleSel.Length() == 0 {
			titleSel = item.Find("h2").First()
		}
		if titleSel.Length() == 0 {
			titleSel = item.Find(".job-title").First()
		}

		title := ExtractText(titleSel)
		if title == "" {
			text := strings.TrimSpace(linkSel.Text())
			if parts := strings.SplitN(text, "\n", 2); len(parts) > 0 {
				title = strings.TrimSpace(parts[0])
			}
		}
		if title == "" {
			return true
		}

		company := ExtractText(item.Find("span.company").First())
		if company == "" {
			company = ExtractText(item.Find(".company-name").First())
		}

		location := ExtractText(item.Find("span.region").First())
		if location == "" {
			location = ExtractText(item.Find(".location").First())
		}
		if location == "" {
			location = "Remote"
		}

		href, _ := linkSel.Attr("href")
		link := href
		if !strings.HasPrefix(href, "http") {
			link = "https://weworkremotely.com" + href
		}

		jobs = append(jobs, models.Job{
			Title:    title,
			Company:  company,
			Location: location,
			Link:     link,
			Source:   "We Work Remotely",
			WorkType: "Remote",
		})
		return true
	})

	slog.Info("parsed jobs", "source", "We Work Remotely", "count", len(jobs))
	return jobs, nil
}

func (p *weWorkRemotelyParser) ParseDetails(html string) (map[string]string, error) {
	details := map[string]string{
		"description":      "",
		"skills":           "",
		"responsibilities": "",
		"benefits":         "",
		"work_type":        "Remote",
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return details, nil
	}

	var descSel *goquery.Selection
	for _, sel := range []string{".listing-container", ".job-description", ".content", "article"} {
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

	return details, nil
}
