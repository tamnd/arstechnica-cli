package arstechnica

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// rssXML returns a minimal valid RSS 2.0 feed with the given items injected.
func rssXML(items string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:dc="http://purl.org/dc/elements/1.1/">
  <channel>
` + items + `
  </channel>
</rss>`
}

func singleItem(title, link, pubDate, creator, category, description string) string {
	return `<item>
  <title>` + title + `</title>
  <link>` + link + `</link>
  <pubDate>` + pubDate + `</pubDate>
  <dc:creator><![CDATA[` + creator + `]]></dc:creator>
  <category><![CDATA[` + category + `]]></category>
  <description><![CDATA[` + description + `]]></description>
</item>`
}

func newTestClient(ts *httptest.Server) *Client {
	cfg := DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	return NewClient(cfg)
}

func TestArticlesParsesTitle(t *testing.T) {
	xml := rssXML(singleItem(
		"Quantum Leap for GPUs",
		"https://arstechnica.com/technology/2024/01/quantum-leap/",
		"Mon, 15 Jan 2024 12:00:00 +0000",
		"Jane Smith",
		"Technology",
		"<p>A short summary.</p>",
	))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer ts.Close()

	arts, err := newTestClient(ts).Articles(context.Background(), "technology", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 1 {
		t.Fatalf("got %d articles, want 1", len(arts))
	}
	if arts[0].Title != "Quantum Leap for GPUs" {
		t.Errorf("Title = %q", arts[0].Title)
	}
}

func TestArticlesParsesAuthor(t *testing.T) {
	xml := rssXML(singleItem(
		"New Telescope Data",
		"https://arstechnica.com/science/2024/01/telescope/",
		"Wed, 10 Jan 2024 09:00:00 +0000",
		"Jennifer Ouellette",
		"Science",
		"<p>Body text.</p>",
	))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer ts.Close()

	arts, err := newTestClient(ts).Articles(context.Background(), "science", 0)
	if err != nil {
		t.Fatal(err)
	}
	if arts[0].Author != "Jennifer Ouellette" {
		t.Errorf("Author = %q", arts[0].Author)
	}
}

func TestArticlesParsesURL(t *testing.T) {
	wantURL := "https://arstechnica.com/gaming/2024/01/new-game/"
	xml := rssXML(singleItem(
		"New Game Released",
		wantURL,
		"Fri, 12 Jan 2024 15:30:00 +0000",
		"Sam Machkovech",
		"Gaming",
		"<p>Summary here.</p>",
	))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer ts.Close()

	arts, err := newTestClient(ts).Articles(context.Background(), "gaming", 0)
	if err != nil {
		t.Fatal(err)
	}
	if arts[0].URL != wantURL {
		t.Errorf("URL = %q, want %q", arts[0].URL, wantURL)
	}
}

func TestArticlesParsesDate(t *testing.T) {
	xml := rssXML(singleItem(
		"Security Flaw Found",
		"https://arstechnica.com/security/2024/03/flaw/",
		"Thu, 07 Mar 2024 18:00:00 +0000",
		"Dan Goodin",
		"Security",
		"<p>Details.</p>",
	))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer ts.Close()

	arts, err := newTestClient(ts).Articles(context.Background(), "security", 0)
	if err != nil {
		t.Fatal(err)
	}
	if arts[0].Published != "2024-03-07" {
		t.Errorf("Published = %q, want %q", arts[0].Published, "2024-03-07")
	}
}

func TestArticlesStripsSummaryHTML(t *testing.T) {
	xml := rssXML(singleItem(
		"Policy Update",
		"https://arstechnica.com/tech-policy/2024/01/policy/",
		"Sat, 20 Jan 2024 10:00:00 +0000",
		"Kate Cox",
		"Policy",
		"<p>This is the <b>summary</b> text.</p>",
	))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer ts.Close()

	arts, err := newTestClient(ts).Articles(context.Background(), "policy", 0)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(arts[0].Summary, "<") || strings.Contains(arts[0].Summary, ">") {
		t.Errorf("Summary contains HTML tags: %q", arts[0].Summary)
	}
	if !strings.Contains(arts[0].Summary, "summary") {
		t.Errorf("Summary text missing: %q", arts[0].Summary)
	}
}

func TestArticlesTruncatesSummary(t *testing.T) {
	long := strings.Repeat("x", 300)
	xml := rssXML(singleItem(
		"Long Article",
		"https://arstechnica.com/technology/2024/01/long/",
		"Mon, 01 Jan 2024 00:00:00 +0000",
		"Author Name",
		"Technology",
		"<p>"+long+"</p>",
	))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer ts.Close()

	arts, err := newTestClient(ts).Articles(context.Background(), "technology", 0)
	if err != nil {
		t.Fatal(err)
	}
	runes := []rune(arts[0].Summary)
	if len(runes) > 150 {
		t.Errorf("Summary too long: %d runes", len(runes))
	}
	if !strings.HasSuffix(arts[0].Summary, "…") {
		t.Errorf("Summary missing ellipsis: %q", arts[0].Summary)
	}
}

func TestArticlesRankOrder(t *testing.T) {
	items := singleItem("A", "https://arstechnica.com/science/2024/01/a/", "Mon, 01 Jan 2024 00:00:00 +0000", "X", "Science", "") +
		singleItem("B", "https://arstechnica.com/science/2024/01/b/", "Tue, 02 Jan 2024 00:00:00 +0000", "Y", "Science", "") +
		singleItem("C", "https://arstechnica.com/science/2024/01/c/", "Wed, 03 Jan 2024 00:00:00 +0000", "Z", "Science", "")
	xml := rssXML(items)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer ts.Close()

	arts, err := newTestClient(ts).Articles(context.Background(), "science", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 3 {
		t.Fatalf("got %d articles, want 3", len(arts))
	}
	for i, a := range arts {
		if a.Rank != i+1 {
			t.Errorf("arts[%d].Rank = %d, want %d", i, a.Rank, i+1)
		}
	}
}

func TestArticlesLimit(t *testing.T) {
	items := ""
	for i := 0; i < 5; i++ {
		items += singleItem("T", "https://arstechnica.com/gaming/2024/01/x/", "Mon, 01 Jan 2024 00:00:00 +0000", "A", "Gaming", "")
	}
	xml := rssXML(items)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer ts.Close()

	arts, err := newTestClient(ts).Articles(context.Background(), "gaming", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 2 {
		t.Errorf("got %d articles with limit=2, want 2", len(arts))
	}
}

func TestArticlesUnknownSection(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	_, err := newTestClient(ts).Articles(context.Background(), "nonexistent", 0)
	if !errors.Is(err, ErrUnknownSection) {
		t.Errorf("got %v, want ErrUnknownSection", err)
	}
}

func TestArticlesSectionFromCategory(t *testing.T) {
	xml := rssXML(singleItem(
		"Cars Article",
		"https://arstechnica.com/cars/2024/01/car-review/",
		"Fri, 05 Jan 2024 08:00:00 +0000",
		"Aurich Lawson",
		"Cars",
		"<p>Summary.</p>",
	))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer ts.Close()

	arts, err := newTestClient(ts).Articles(context.Background(), "cars", 0)
	if err != nil {
		t.Fatal(err)
	}
	if arts[0].Section != "cars" {
		t.Errorf("Section = %q, want %q", arts[0].Section, "cars")
	}
}

func TestSectionsReturnsAll(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(rssXML("")))
	}))
	defer ts.Close()

	secs := newTestClient(ts).Sections()
	if len(secs) != 8 {
		t.Errorf("got %d sections, want 8", len(secs))
	}
	for i, s := range secs {
		if s.Rank != i+1 {
			t.Errorf("secs[%d].Rank = %d, want %d", i, s.Rank, i+1)
		}
		if s.Name == "" {
			t.Errorf("secs[%d].Name is empty", i)
		}
		if !strings.HasPrefix(s.URL, ts.URL) {
			t.Errorf("secs[%d].URL = %q, should start with test server URL", i, s.URL)
		}
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(rssXML("")))
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	cfg.Retries = 5
	c := NewClient(cfg)

	start := time.Now()
	_, err := c.get(context.Background(), ts.URL+"/arstechnica/index")
	if err != nil {
		t.Fatal(err)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

func TestGetUserAgent(t *testing.T) {
	var gotUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(rssXML("")))
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	c := NewClient(cfg)
	_, _ = c.get(context.Background(), ts.URL+"/arstechnica/index")

	if gotUA == "" {
		t.Error("request carried no User-Agent")
	}
}
