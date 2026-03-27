package scraper

import (
	"strings"
	"testing"

	"github.com/campbell/huntr-ai/internal/config"
)

func TestBuildSearchURL_UsesKeywordsForIndeed(t *testing.T) {
	url := BuildSearchURL("https://uk.indeed.com/jobs?q=foo&l=United+Kingdom", "Indeed", []string{"Go developer remote"})
	if !strings.Contains(strings.ToLower(url), "go+developer+remote") {
		t.Fatalf("expected query terms in URL, got: %s", url)
	}
}

func TestSourceKeywordsForSearch_UsesConfiguredSourceKeywords(t *testing.T) {
	src := config.Source{Name: "Indeed", Keywords: []string{"explicit", "keywords"}}
	prefs := config.Preferences{}
	keywords := sourceKeywordsForSearch(src, prefs, "indeed")
	if len(keywords) != 2 || keywords[0] != "explicit" {
		t.Fatalf("expected source keywords to win, got: %#v", keywords)
	}
}

func TestSourceKeywordsForSearch_UsesRoleProfileQueryTerms(t *testing.T) {
	src := config.Source{Name: "Indeed"}
	prefs := config.Preferences{
		RoleProfile: config.RoleProfile{
			QueryTerms: []string{"Go developer remote", "Platform engineer remote", "DevOps engineer remote"},
		},
	}
	keywords := sourceKeywordsForSearch(src, prefs, "indeed")
	if len(keywords) == 0 {
		t.Fatal("expected derived keywords")
	}
	for _, kw := range keywords {
		if kw == "" {
			t.Fatal("expected non-empty derived keyword")
		}
	}
}
