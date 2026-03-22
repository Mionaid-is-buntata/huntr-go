package parsers

import (
	"github.com/campbell/huntr-ai/internal/models"
)

func init() {
	// Register all recruitment agencies that use the generic parser.
	agencies := []string{
		"InterQuest",
		"Understanding Recruitment",
		"SWIFTRA",
		"SThree",
		"Flexa Careers",
		"Oliver Bernard",
		"Explore Group",
		"Spectrum IT",
		"Burns Sheehan",
		"ECOM Recruitment",
	}

	for _, name := range agencies {
		Register(name, &agencyParser{sourceName: name})
	}

	// Extra aliases for common variants
	RegisterAlias("interquest group", "interquest")
	RegisterAlias("flexa", "flexacareers")
	RegisterAlias("spectrum", "spectrumit")
	RegisterAlias("ecom", "ecomrecruitment")
}

// agencyParser wraps the generic parser with a fixed source name.
type agencyParser struct {
	sourceName string
}

func (p *agencyParser) Name() string { return p.sourceName }

func (p *agencyParser) ParseListings(html string, sourceURL string) ([]models.Job, error) {
	return parseGenericListings(html, sourceURL, p.sourceName)
}

func (p *agencyParser) ParseDetails(html string) (map[string]string, error) {
	return ParseGenericDetails(html)
}
