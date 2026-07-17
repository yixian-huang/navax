package seo

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"
)

// URLEntry is one <url> row in a sitemap.
type URLEntry struct {
	Loc        string
	LastMod    time.Time
	ChangeFreq string
	Priority   string
}

// RobotsTxt returns a conservative robots.txt body.
func RobotsTxt(publicBaseURL string, discoverEnabled bool) string {
	base := strings.TrimRight(strings.TrimSpace(publicBaseURL), "/")
	var b strings.Builder
	b.WriteString("# nav.ax robots.txt\n")
	b.WriteString("User-agent: *\n")
	b.WriteString("Allow: /\n")
	b.WriteString("Allow: /discover\n")
	b.WriteString("Allow: /u/\n")
	b.WriteString("Allow: /privacy\n")
	b.WriteString("Allow: /terms\n")
	b.WriteString("Allow: /cookies\n")
	b.WriteString("Disallow: /app\n")
	b.WriteString("Disallow: /app/\n")
	b.WriteString("Disallow: /admin\n")
	b.WriteString("Disallow: /admin/\n")
	b.WriteString("Disallow: /api/\n")
	b.WriteString("Disallow: /login\n")
	b.WriteString("Disallow: /register\n")
	b.WriteString("Disallow: /invite\n")
	b.WriteString("Disallow: /setup\n")
	b.WriteString("Disallow: /forgot-password\n")
	b.WriteString("Disallow: /reset-password\n")
	if !discoverEnabled {
		b.WriteString("Disallow: /discover\n")
	}
	if base != "" {
		b.WriteString("Sitemap: ")
		b.WriteString(base)
		b.WriteString("/sitemap.xml\n")
	}
	return b.String()
}

type urlset struct {
	XMLName xml.Name     `xml:"urlset"`
	Xmlns   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc        string `xml:"loc"`
	LastMod    string `xml:"lastmod,omitempty"`
	ChangeFreq string `xml:"changefreq,omitempty"`
	Priority   string `xml:"priority,omitempty"`
}

// WriteSitemapXML writes a urlset document.
func WriteSitemapXML(w io.Writer, entries []URLEntry) error {
	set := urlset{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  make([]sitemapURL, 0, len(entries)),
	}
	for _, entry := range entries {
		loc := strings.TrimSpace(entry.Loc)
		if loc == "" {
			continue
		}
		item := sitemapURL{
			Loc:        loc,
			ChangeFreq: entry.ChangeFreq,
			Priority:   entry.Priority,
		}
		if !entry.LastMod.IsZero() {
			item.LastMod = entry.LastMod.UTC().Format("2006-01-02")
		}
		set.URLs = append(set.URLs, item)
	}
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(set); err != nil {
		return fmt.Errorf("encode sitemap: %w", err)
	}
	if _, err := io.WriteString(w, "\n"); err != nil {
		return err
	}
	return nil
}

// BuildDefaultEntries composes home + discover + legal + public share URLs.
func BuildDefaultEntries(publicBaseURL string, discoverEnabled bool, publicPages []URLEntry) []URLEntry {
	base := strings.TrimRight(strings.TrimSpace(publicBaseURL), "/")
	now := time.Now().UTC()
	entries := []URLEntry{
		{Loc: base + "/", LastMod: now, ChangeFreq: "daily", Priority: "1.0"},
		{Loc: base + "/privacy", ChangeFreq: "yearly", Priority: "0.3"},
		{Loc: base + "/terms", ChangeFreq: "yearly", Priority: "0.3"},
		{Loc: base + "/cookies", ChangeFreq: "yearly", Priority: "0.3"},
	}
	if discoverEnabled {
		entries = append(entries, URLEntry{
			Loc: base + "/discover", LastMod: now, ChangeFreq: "daily", Priority: "0.8",
		})
	}
	for _, page := range publicPages {
		loc := strings.TrimSpace(page.Loc)
		if loc == "" {
			continue
		}
		if !strings.HasPrefix(loc, "http://") && !strings.HasPrefix(loc, "https://") {
			if !strings.HasPrefix(loc, "/") {
				loc = "/u/" + loc
			}
			loc = base + loc
		}
		entries = append(entries, URLEntry{
			Loc:        loc,
			LastMod:    page.LastMod,
			ChangeFreq: "weekly",
			Priority:   "0.6",
		})
	}
	return entries
}
