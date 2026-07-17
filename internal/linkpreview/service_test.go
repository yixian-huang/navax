package linkpreview

import (
	"net/url"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestNormalizeURL(t *testing.T) {
	got, err := normalizeURL("github.com")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://github.com" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractMeta(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`<!doctype html><html><head>
<title>Example Site</title>
<meta name="description" content="A demo page">
<meta property="og:title" content="OG Example">
<link rel="icon" href="/favicon.ico">
</head><body></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	meta := extractMeta(doc)
	if meta.title != "Example Site" {
		t.Fatalf("title = %q", meta.title)
	}
	if meta.description != "A demo page" {
		t.Fatalf("description = %q", meta.description)
	}
	if meta.ogTitle != "OG Example" {
		t.Fatalf("ogTitle = %q", meta.ogTitle)
	}
	if meta.icon != "/favicon.ico" {
		t.Fatalf("icon = %q", meta.icon)
	}
}

func TestSoftPreview(t *testing.T) {
	u, err := url.Parse("https://www.example.com/path")
	if err != nil {
		t.Fatal(err)
	}
	p := softPreview(u)
	if p.Title != "Example" {
		t.Fatalf("title = %q", p.Title)
	}
	if !strings.Contains(p.FaviconURL, "example.com") {
		t.Fatalf("favicon = %q", p.FaviconURL)
	}
}

func TestDomainTitle(t *testing.T) {
	if domainTitle("www.github.com") != "Github" {
		t.Fatalf("got %q", domainTitle("www.github.com"))
	}
}

func TestResolveURL(t *testing.T) {
	base, _ := url.Parse("https://example.com/app/")
	got := resolveURL(base, "/icon.png")
	if got != "https://example.com/icon.png" {
		t.Fatalf("got %q", got)
	}
}
