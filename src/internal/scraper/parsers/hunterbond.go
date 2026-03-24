package parsers

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/campbell/huntr-ai/internal/models"
)

func init() {
	Register("Hunter Bond", &hunterBondParser{})
	RegisterAlias("hunterbond", "hunterbond")
}

type hunterBondParser struct{}

func (p *hunterBondParser) Name() string { return "Hunter Bond" }

func (p *hunterBondParser) ParseListings(html string, sourceURL string) ([]models.Job, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("scraper: Hunter Bond parse: %w", err)
	}

	var jobs []models.Job

	doc.Find("div.job-data").Each(func(_ int, card *goquery.Selection) {
		// Title from h4.xs-heading > a
		titleSel := card.Find("h4.xs-heading a").First()
		title := strings.TrimSpace(titleSel.Text())
		if title == "" {
			return
		}

		// Link: the h4 > a href is often empty; fall back to data-slug
		href, _ := titleSel.Attr("href")
		if href == "" || href == "#" {
			slug, exists := card.Find("[data-slug]").Attr("data-slug")
			if exists && slug != "" {
				href = fmt.Sprintf("/jobs/%s/", slug)
			}
		}
		link := ResolveURL(sourceURL, href)

		// Extract highlights from ul.job-data-highlights > li
		var salary, location, workType string
		card.Find("ul.job-data-highlights li").Each(func(_ int, li *goquery.Selection) {
			icon := li.Find("i").First()
			cls, _ := icon.Attr("class")
			text := strings.TrimSpace(li.Find("span").Text())

			switch {
			case strings.Contains(cls, "money"):
				salary = text
			case strings.Contains(cls, "location"):
				location = text
			case strings.Contains(cls, "type"):
				workType = strings.TrimSpace(text)
			}
		})

		if location == "" {
			location = "UK"
		}

		jobs = append(jobs, models.Job{
			Title:    title,
			Company:  "Hunter Bond",
			Location: location,
			Salary:   salary,
			WorkType: workType,
			Link:     link,
			Source:   "Hunter Bond",
		})
	})

	return jobs, nil
}
