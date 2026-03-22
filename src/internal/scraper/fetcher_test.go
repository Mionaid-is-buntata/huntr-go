package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchStatic_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html><body>Hello</body></html>"))
	}))
	defer srv.Close()

	f := NewFetcher()
	html, err := f.FetchStatic(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if html != "<html><body>Hello</body></html>" {
		t.Errorf("unexpected body: %s", html)
	}
}

func TestFetchStatic_403SetsCooldown(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	f := NewFetcher()
	_, err := f.FetchStatic(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error for 403")
	}

	// Should have retried MaxRetries+1 times
	if attempts != MaxRetries+1 {
		t.Errorf("expected %d attempts, got %d", MaxRetries+1, attempts)
	}

	// Domain should now be in cooldown
	domain := getDomain(srv.URL)
	if !f.IsDomainCooledDown(domain) {
		t.Error("expected domain to be in cooldown")
	}
}

func TestFetchStatic_CooldownBlocks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	f := NewFetcher()

	// First call: triggers cooldown
	_, _ = f.FetchStatic(context.Background(), srv.URL)

	// Second call: should fail immediately with cooldown error
	_, err := f.FetchStatic(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected cooldown error")
	}
	if !f.IsDomainCooledDown(getDomain(srv.URL)) {
		t.Error("domain should still be in cooldown")
	}
}

func TestFetchStatic_UARotation(t *testing.T) {
	var agents []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agents = append(agents, r.Header.Get("User-Agent"))
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	f := NewFetcher()
	_, _ = f.FetchStatic(context.Background(), srv.URL)
	_, _ = f.FetchStatic(context.Background(), srv.URL)

	if len(agents) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(agents))
	}
	if agents[0] == agents[1] {
		t.Error("expected different user agents on consecutive requests")
	}
}

func TestGetDomain(t *testing.T) {
	tests := []struct {
		url    string
		domain string
	}{
		{"https://www.reed.co.uk/jobs/python", "www.reed.co.uk"},
		{"https://uk.indeed.com/jobs?q=test", "uk.indeed.com"},
		{"http://localhost:8080/test", "localhost:8080"},
	}

	for _, tt := range tests {
		got := getDomain(tt.url)
		if got != tt.domain {
			t.Errorf("getDomain(%q) = %q, want %q", tt.url, got, tt.domain)
		}
	}
}

func TestAddJitter(t *testing.T) {
	for i := 0; i < 100; i++ {
		result := addJitter(3.0)
		if result < 0.5 {
			t.Errorf("jitter result %f below minimum 0.5", result)
		}
		if result > 6.0 {
			t.Errorf("jitter result %f unexpectedly high", result)
		}
	}
}
