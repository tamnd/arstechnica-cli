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

// ─── XML wire types ──────────────────────────────────────────────────────────

type atomFeed struct {
	XMLName xml.Name    `xml:"feed"`
	Entries []atomEntry `xml:"entry"`
}

type atomEntry struct {
	Title     string     `xml:"title"`
	Links     []atomLink `xml:"link"`
	Published string     `xml:"published"`
	Updated   string     `xml:"updated"`
	Author    struct {
		Name string `xml:"name"`
	} `xml:"author"`
	Subject string `xml:"subject"`
	Summary string `xml:"summary"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func pickLink(links []atomLink) string {
	for _, l := range links {
		if l.Rel == "" || l.Rel == "alternate" {
			return l.Href
		}
	}
	if len(links) > 0 {
		return links[0].Href
	}
	return ""
}

func parseDate(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return t.UTC().Format("2006-01-02")
}

// sectionFromURL extracts the first non-empty path segment after the host.
// e.g. "https://arstechnica.com/science/2024/01/slug/" -> "science"
func sectionFromURL(u string) string {
	// strip scheme
	rest := u
	if idx := strings.Index(rest, "://"); idx >= 0 {
		rest = rest[idx+3:]
	}
	// strip host
	if idx := strings.Index(rest, "/"); idx >= 0 {
		rest = rest[idx+1:]
	} else {
		return ""
	}
	// first path segment
	seg := rest
	if idx := strings.Index(seg, "/"); idx >= 0 {
		seg = seg[:idx]
	}
	return seg
}

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

func entryToArticle(e atomEntry, rank int) Article {
	u := pickLink(e.Links)
	section := e.Subject
	if section == "" {
		section = sectionFromURL(u)
	}
	published := e.Published
	if published == "" {
		published = e.Updated
	}
	return Article{
		Rank:      rank,
		Title:     strings.TrimSpace(e.Title),
		Author:    strings.TrimSpace(e.Author.Name),
		Section:   strings.ToLower(strings.TrimSpace(section)),
		Published: parseDate(published),
		Summary:   stripAndTruncate(e.Summary, 150),
		URL:       u,
	}
}
