package scraper

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// CooldownMinutes is the cooldown period after a 403 response.
	CooldownMinutes = 15
	// MaxRetries is the number of retry attempts for static fetches.
	MaxRetries = 2
	// RequestTimeout is the default HTTP request timeout.
	RequestTimeout = 30 * time.Second
	// DynamicRenderWait is the time to allow JS rendering after page load.
	DynamicRenderWait = 3 * time.Second
	// MaxRedirects is the maximum number of HTTP redirects to follow.
	MaxRedirects = 5
	// MaxResponseBytes is the maximum HTTP response body size (10MB).
	MaxResponseBytes = 10 << 20
)

// userAgents is rotated per-request to avoid detection.
var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:122.0) Gecko/20100101 Firefox/122.0",
	"Mozilla/5.0 (X11; Linux x86_64; rv:122.0) Gecko/20100101 Firefox/122.0",
}

// googleBlockDomains are blocked at Chrome resolver level to prevent
// DNS hammering of accounts.google.com etc. on the Pi.
var googleBlockDomains = []string{
	"accounts.google.com", "clients1.google.com", "clients2.google.com",
	"clients3.google.com", "clients4.google.com", "clients5.google.com",
	"clients6.google.com", "clients.google.com", "www.google.com",
	"google.com", "apis.google.com", "oauth.google.com",
	"fonts.googleapis.com", "fonts.gstatic.com", "ssl.gstatic.com",
	"www.gstatic.com", "gstatic.com", "www.google-analytics.com",
	"google-analytics.com", "www.googletagmanager.com", "googletagmanager.com",
	"googleadservices.com", "pagead2.googlesyndication.com",
	"tpc.googlesyndication.com", "safebrowsing.googleapis.com",
}

// PageFetcher abstracts page fetching for testability.
type PageFetcher interface {
	FetchStatic(ctx context.Context, url string) (string, error)
	FetchDynamic(ctx context.Context, url string) (string, error)
	Close()
}

// Fetcher implements PageFetcher with HTTP and rod-based Chrome fetching.
type Fetcher struct {
	client     *http.Client
	cooldowns  map[string]time.Time // domain -> cooldown-until
	cooldownMu sync.RWMutex
	uaIndex    atomic.Int64
}

// NewFetcher creates a new Fetcher with an HTTP client.
func NewFetcher() *Fetcher {
	return &Fetcher{
		client: &http.Client{
			Timeout: RequestTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= MaxRedirects {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		cooldowns: make(map[string]time.Time),
	}
}

// FetchStatic fetches a page using net/http with UA rotation, domain cooldown,
// and exponential backoff on 429.
func (f *Fetcher) FetchStatic(ctx context.Context, fetchURL string) (string, error) {
	domain := getDomain(fetchURL)

	// Check cooldown
	f.cooldownMu.RLock()
	if until, ok := f.cooldowns[domain]; ok && time.Now().Before(until) {
		remaining := time.Until(until).Truncate(time.Minute)
		f.cooldownMu.RUnlock()
		return "", fmt.Errorf("domain %s in cooldown for %v", domain, remaining)
	}
	f.cooldownMu.RUnlock()

	var lastErr error
	for attempt := 0; attempt <= MaxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		// Backoff with jitter between retries
		if attempt > 0 {
			delay := addJitter(3.0 * float64(attempt))
			slog.Info("retrying fetch", "url", fetchURL, "attempt", attempt+1, "delay", delay)
			select {
			case <-time.After(time.Duration(delay * float64(time.Second))):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fetchURL, nil)
		if err != nil {
			return "", fmt.Errorf("scraper: create request: %w", err)
		}

		f.setHeaders(req)

		resp, err := f.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("scraper: fetch %s: %w", fetchURL, err)
			continue
		}

		body, readErr := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBytes))
		resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("scraper: read body: %w", readErr)
			continue
		}

		switch {
		case resp.StatusCode == 403:
			if attempt == MaxRetries {
				f.setCooldown(domain)
				return "", fmt.Errorf("scraper: 403 forbidden for %s after %d attempts, cooldown set", fetchURL, MaxRetries+1)
			}
			lastErr = fmt.Errorf("scraper: 403 forbidden for %s", fetchURL)
			continue

		case resp.StatusCode == 429:
			if attempt == MaxRetries {
				return "", fmt.Errorf("scraper: 429 rate limited for %s after %d attempts", fetchURL, MaxRetries+1)
			}
			lastErr = fmt.Errorf("scraper: 429 rate limited for %s", fetchURL)
			continue

		case resp.StatusCode >= 400:
			return "", fmt.Errorf("scraper: HTTP %d for %s", resp.StatusCode, fetchURL)

		default:
			slog.Debug("fetched page", "url", fetchURL, "bytes", len(body))
			return string(body), nil
		}
	}

	return "", lastErr
}

// FetchDynamic fetches a page using rod headless Chrome with JS rendering.
// Rod browser is lazily initialised on first call.
func (f *Fetcher) FetchDynamic(ctx context.Context, fetchURL string) (string, error) {
	// TODO: Implement rod-based dynamic fetching in Phase 4/5 integration.
	// For now, fall back to static fetch with a warning.
	slog.Warn("dynamic fetch not yet implemented, falling back to static", "url", fetchURL)
	return f.FetchStatic(ctx, fetchURL)
}

// Close cleans up any open browser instances.
func (f *Fetcher) Close() {
	// TODO: Close rod browser when implemented.
}

// IsDomainCooledDown checks if a domain is in cooldown.
func (f *Fetcher) IsDomainCooledDown(domain string) bool {
	f.cooldownMu.RLock()
	defer f.cooldownMu.RUnlock()
	until, ok := f.cooldowns[domain]
	return ok && time.Now().Before(until)
}

func (f *Fetcher) setHeaders(req *http.Request) {
	idx := f.uaIndex.Add(1)
	ua := userAgents[int(idx)%len(userAgents)]

	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Cache-Control", "max-age=0")
}

func (f *Fetcher) setCooldown(domain string) {
	f.cooldownMu.Lock()
	defer f.cooldownMu.Unlock()
	until := time.Now().Add(CooldownMinutes * time.Minute)
	f.cooldowns[domain] = until
	slog.Warn("domain cooldown set", "domain", domain, "until", until.Format("15:04:05"))
}

func getDomain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return u.Host
}

func addJitter(baseDelay float64) float64 {
	jitter := baseDelay * 0.3 * (rand.Float64()*2 - 1)
	result := baseDelay + jitter
	if result < 0.5 {
		result = 0.5
	}
	return result
}

// GoogleHostResolverRules returns the Chrome flag value to block Google domains.
func GoogleHostResolverRules() string {
	parts := make([]string, len(googleBlockDomains))
	for i, d := range googleBlockDomains {
		parts[i] = fmt.Sprintf("MAP %s 0.0.0.0", d)
	}
	return strings.Join(parts, ",")
}
