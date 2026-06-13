// Package arstechnica is the library behind the ars command: the HTTP client,
// request shaping, and the typed data models for Ars Technica.
//
// Data comes from the public Atom RSS feeds at feeds.arstechnica.com. No API
// key is required. The client sends a real User-Agent, paces requests, and
// retries 429/5xx with exponential backoff.
package arstechnica

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ErrUnknownSection is returned when the section argument does not match any
// registered feed key.
var ErrUnknownSection = errors.New("unknown section")

// feedEntry maps a section key to its feed path (relative to BaseURL).
type feedEntry struct {
	key   string
	path  string
	label string
}

var feeds = []feedEntry{
	{"index", "/arstechnica/index", "All articles"},
	{"technology", "/arstechnica/technology-lab", "Technology"},
	{"science", "/arstechnica/science", "Science"},
	{"gaming", "/arstechnica/gaming", "Gaming"},
	{"policy", "/arstechnica/tech-policy", "Tech policy"},
	{"security", "/arstechnica/security", "Security"},
	{"business", "/arstechnica/business", "Business"},
	{"cars", "/arstechnica/cars", "Cars"},
}

// Config holds constructor parameters for Client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
}

// DefaultConfig returns sensible production defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "http://feeds.arstechnica.com",
		UserAgent: "ars/dev (+https://github.com/tamnd/arstechnica-cli)",
		Rate:      500 * time.Millisecond,
		Retries:   3,
		Timeout:   30 * time.Second,
	}
}

// Client fetches Ars Technica Atom feeds.
type Client struct {
	httpClient *http.Client
	baseURL    string
	userAgent  string
	rate       time.Duration
	retries    int
	mu         sync.Mutex
	last       time.Time
}

// NewClient returns a Client configured by cfg.
func NewClient(cfg Config) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: cfg.Timeout},
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		userAgent:  cfg.UserAgent,
		rate:       cfg.Rate,
		retries:    cfg.Retries,
	}
}

// Articles fetches the feed for the given section key and returns up to limit
// articles ranked by feed order. limit=0 returns all entries.
// Returns ErrUnknownSection if the section key is not registered.
func (c *Client) Articles(ctx context.Context, section string, limit int) ([]Article, error) {
	fe, ok := feedByKey(section)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownSection, section)
	}
	rawURL := c.baseURL + fe.path
	body, err := c.get(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	var feed atomFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("parse feed %s: %w", rawURL, err)
	}
	entries := feed.Entries
	if limit > 0 && limit < len(entries) {
		entries = entries[:limit]
	}
	out := make([]Article, len(entries))
	for i, e := range entries {
		out[i] = entryToArticle(e, i+1)
	}
	return out, nil
}

// Sections returns the static list of registered feed sections.
// No network request is made.
func (c *Client) Sections() []Section {
	out := make([]Section, len(feeds))
	for i, fe := range feeds {
		out[i] = Section{
			Rank: i + 1,
			Name: fe.key,
			URL:  c.baseURL + fe.path,
		}
	}
	return out
}

// feedByKey looks up a feed entry by its section key (case-insensitive).
func feedByKey(key string) (feedEntry, bool) {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, fe := range feeds {
		if fe.key == key {
			return fe, true
		}
	}
	return feedEntry{}, false
}

// get fetches a URL with pacing and retries.
func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/xml")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rate <= 0 {
		return
	}
	if wait := c.rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}
