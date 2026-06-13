package arstechnica

import (
	"encoding/xml"
	"strings"
	"time"
)

// Article is the record emitted for Ars Technica articles.
type Article struct {
	Rank      int    `json:"rank"`
	Title     string `json:"title"`
	Author    string `json:"author"`
	Section   string `json:"section"`
	Published string `json:"published"`
	Summary   string `json:"summary"`
	URL       string `json:"url"`
}

// Section is the record emitted by the sections command.
type Section struct {
	Rank int    `json:"rank"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// ─── RSS 2.0 wire types ───────────────────────────────────────────────────────

// rssFeed is the root of an RSS 2.0 document from feeds.arstechnica.com.
type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

// rssItem maps to each <item> in the feed.
// dc:creator maps to the local name "creator" (encoding/xml matches local name).
type rssItem struct {
	Title       string   `xml:"title"`
	Link        string   `xml:"link"`
	PubDate     string   `xml:"pubDate"`
	Creator     string   `xml:"creator"`
	Description string   `xml:"description"`
	Categories  []string `xml:"category"`
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// parseDate parses an RSS pubDate (RFC1123Z: "Mon, 02 Jan 2006 15:04:05 +0000")
// and returns "2006-01-02". Falls back to the raw string on parse error.
func parseDate(s string) string {
	s = strings.TrimSpace(s)
	t, err := time.Parse(time.RFC1123Z, s)
	if err != nil {
		// try RFC1123 without timezone offset
		t, err = time.Parse(time.RFC1123, s)
		if err != nil {
			return s
		}
	}
	return t.UTC().Format("2006-01-02")
}

// sectionFromURL extracts the first non-empty path segment after the host.
// e.g. "https://arstechnica.com/science/2024/01/slug/" -> "science"
func sectionFromURL(u string) string {
	rest := u
	if idx := strings.Index(rest, "://"); idx >= 0 {
		rest = rest[idx+3:]
	}
	if idx := strings.Index(rest, "/"); idx >= 0 {
		rest = rest[idx+1:]
	} else {
		return ""
	}
	seg := rest
	if idx := strings.Index(seg, "/"); idx >= 0 {
		seg = seg[:idx]
	}
	return strings.ToLower(seg)
}

// stripAndTruncate strips HTML tags, collapses common entities, and truncates
// to maxChars runes, appending "…" if truncated.
func stripAndTruncate(html string, maxChars int) string {
	var b strings.Builder
	inTag := false
	for _, r := range html {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	out := b.String()
	out = strings.ReplaceAll(out, "&amp;", "&")
	out = strings.ReplaceAll(out, "&lt;", "<")
	out = strings.ReplaceAll(out, "&gt;", ">")
	out = strings.ReplaceAll(out, "&quot;", `"`)
	out = strings.ReplaceAll(out, "&#39;", "'")
	out = strings.ReplaceAll(out, "&apos;", "'")
	out = strings.TrimSpace(out)
	rs := []rune(out)
	if len(rs) > maxChars {
		return string(rs[:maxChars-1]) + "…"
	}
	return out
}

// sectionFromCategories picks the first category that looks like an editorial
// section name (lowercased, no spaces, known or short enough to be a slug).
// Falls back to URL extraction.
func sectionFromCategories(cats []string, u string) string {
	for _, c := range cats {
		c = strings.ToLower(strings.TrimSpace(c))
		// skip multi-word tags like "fungal networks"
		if c != "" && !strings.Contains(c, " ") {
			return c
		}
	}
	return sectionFromURL(u)
}

func itemToArticle(it rssItem, rank int) Article {
	section := sectionFromCategories(it.Categories, it.Link)
	return Article{
		Rank:      rank,
		Title:     strings.TrimSpace(it.Title),
		Author:    strings.TrimSpace(it.Creator),
		Section:   section,
		Published: parseDate(it.PubDate),
		Summary:   stripAndTruncate(it.Description, 150),
		URL:       strings.TrimSpace(it.Link),
	}
}
