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

// atomXML returns a minimal valid Atom feed with the given entries injected.
func atomXML(entries string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:dc="http://purl.org/dc/elements/1.1/">
` + entries + `
</feed>`
}

func singleEntry(title, author, href, published, subject, summary string) string {
	return `<entry>
  <title>` + title + `</title>
  <link href="` + href + `"/>
  <published>` + published + `</published>
  <author><name>` + author + `</name></author>
  <dc:subject>` + subject + `</dc:subject>
  <summary type="html"><![CDATA[` + summary + `]]></summary>
</entry>`
}

func newTestClient(ts *httptest.Server) *Client {
	cfg := DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	return NewClient(cfg)
}

func TestArticlesParsesTitle(t *testing.T) {
	xml := atomXML(singleEntry(
		"Quantum Leap for GPUs",
		"Jane Smith",
		"https://arstechnica.com/technology/2024/01/quantum-leap/",
		"2024-01-15T12:00:00+00:00",
		"technology",
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
	xml := atomXML(singleEntry(
		"New Telescope Data",
		"Jennifer Ouellette",
		"https://arstechnica.com/science/2024/01/telescope/",
		"2024-01-10T09:00:00+00:00",
		"science",
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
	xml := atomXML(singleEntry(
		"New Game Released",
		"Sam Machkovech",
		wantURL,
		"2024-01-12T15:30:00+00:00",
		"gaming",
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
	xml := atomXML(singleEntry(
		"Security Flaw Found",
		"Dan Goodin",
		"https://arstechnica.com/security/2024/03/flaw/",
		"2024-03-07T18:00:00+00:00",
		"security",
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
	xml := atomXML(singleEntry(
		"Policy Update",
		"Kate Cox",
		"https://arstechnica.com/tech-policy/2024/01/policy/",
		"2024-01-20T10:00:00+00:00",
		"policy",
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
	xml := atomXML(singleEntry(
		"Long Article",
		"Author Name",
		"https://arstechnica.com/technology/2024/01/long/",
		"2024-01-01T00:00:00+00:00",
		"technology",
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
	entries := singleEntry("A", "X", "https://arstechnica.com/science/2024/01/a/", "2024-01-01T00:00:00+00:00", "science", "") +
		singleEntry("B", "Y", "https://arstechnica.com/science/2024/01/b/", "2024-01-02T00:00:00+00:00", "science", "") +
		singleEntry("C", "Z", "https://arstechnica.com/science/2024/01/c/", "2024-01-03T00:00:00+00:00", "science", "")
	xml := atomXML(entries)
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
	entries := ""
	for i := 0; i < 5; i++ {
		entries += singleEntry("T", "A", "https://arstechnica.com/gaming/2024/01/x/", "2024-01-01T00:00:00+00:00", "gaming", "")
	}
	xml := atomXML(entries)
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

func TestArticlesSectionFromDCSubject(t *testing.T) {
	xml := atomXML(singleEntry(
		"Cars Article",
		"Aurich Lawson",
		"https://arstechnica.com/cars/2024/01/car-review/",
		"2024-01-05T08:00:00+00:00",
		"cars",
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
		_, _ = w.Write([]byte(atomXML("")))
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
		_, _ = w.Write([]byte(atomXML("")))
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
		_, _ = w.Write([]byte(atomXML("")))
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
