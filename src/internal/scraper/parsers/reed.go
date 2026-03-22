package parsers

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/campbell/huntr-ai/internal/models"
)

func init() {
	Register("reed", &reedParser{})
	RegisterAlias("totaljobs", "reed")
	RegisterAlias("cv-library", "reed")
	RegisterAlias("cvlibrary", "reed")
}

type reedParser struct{}

func (p *reedParser) Name() string { return "Reed" }

func (p *reedParser) ParseListings(html string, sourceURL string) ([]models.Job, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("scraper: reed parse: %w", err)
	}

	// Reed job card selectors (updated Jan 2026)
	cards := FindByClassContaining(doc.Selection, "div", "jobCard")
	if cards.Length() == 0 {
		cards = doc.Find("article.job-card")
	}
	if cards.Length() == 0 {
		cards = doc.Find("article[data-qa='job-card']")
	}
	if cards.Length() == 0 {
		cards = doc.Find("div.job-result-card")
	}

	slog.Info("found job cards", "source", "Reed", "count", cards.Length())

	var jobs []models.Job
	cards.Each(func(_ int, card *goquery.Selection) {
		// Title: data-qa="job-card-title" or fallback
		titleSel := card.Find("a[data-qa='job-card-title']").First()
		if titleSel.Length() == 0 {
			titleSel = FindByClassContaining(card, "a", "jobTitle").First()
		}
		if titleSel.Length() == 0 {
			titleSel = card.Find("h2").First()
		}
		if titleSel.Length() == 0 {
			return
		}

		title := ExtractText(titleSel)
		if title == "" {
			return
		}

		// Company: div[data-qa="job-posted-by"] > a
		var company string
		postedBy := card.Find("div[data-qa='job-posted-by']").First()
		if postedBy.Length() == 0 {
			postedBy = FindByClassContaining(card, "div", "postedBy").First()
		}
		if postedBy.Length() > 0 {
			companyLink := FindByClassContaining(postedBy, "a", "gtmJobListingPostedBy").First()
			if companyLink.Length() == 0 {
				companyLink = postedBy.Find("a").First()
			}
			company = ExtractText(companyLink)
		}
		if company == "" {
			company = "Unknown"
		}

		// Location
		locSel := card.Find("li[data-qa='job-metadata-location']").First()
		if locSel.Length() == 0 {
			locSel = FindByClassContaining(card, "li", "location").First()
		}
		location := ExtractText(locSel)
		if location == "" {
			location = "Unknown"
		}

		// Salary
		salSel := card.Find("li[data-qa='job-metadata-salary']").First()
		if salSel.Length() == 0 {
			salSel = FindByClassContaining(card, "li", "salary").First()
		}
		salary := ExtractText(salSel)

		// Link
		var href string
		if titleSel.Is("a") {
			href, _ = titleSel.Attr("href")
		}
		if href == "" {
			href, _ = card.Find("a[href]").First().Attr("href")
		}
		link := ResolveURL("https://www.reed.co.uk", href)
		if link == "" {
			link = sourceURL
		}

		jobs = append(jobs, models.Job{
			Title:    title,
			Company:  company,
			Location: location,
			Salary:   salary,
			Link:     link,
			Source:   "Reed",
		})
	})

	slog.Info("parsed jobs", "source", "Reed", "count", len(jobs))
	return jobs, nil
}

func (p *reedParser) ParseDetails(html string) (map[string]string, error) {
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

	// Job description
	descSel := doc.Find("div[data-qa='job-description']").First()
	if descSel.Length() == 0 {
		descSel = FindByClassContaining(doc.Selection, "div", "description").First()
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

	// Work type
	pageText := strings.ToLower(doc.Text())
	if strings.Contains(pageText, "remote") || strings.Contains(pageText, "work from home") {
		if strings.Contains(pageText, "hybrid") {
			details["work_type"] = "Hybrid"
		} else {
			details["work_type"] = "Remote"
		}
	} else if strings.Contains(pageText, "hybrid") {
		details["work_type"] = "Hybrid"
	} else if strings.Contains(pageText, "on-site") || strings.Contains(pageText, "office") {
		details["work_type"] = "On-site"
	}

	// Benefits
	benefitsSel := FindByClassContaining(doc.Selection, "div", "benefits").First()
	if benefitsSel.Length() == 0 {
		benefitsSel = FindByClassContaining(doc.Selection, "ul", "benefits").First()
	}
	if benefitsSel.Length() > 0 {
		b := strings.TrimSpace(benefitsSel.Text())
		if len(b) > 500 {
			b = b[:500]
		}
		details["benefits"] = b
	}

	return details, nil
}
