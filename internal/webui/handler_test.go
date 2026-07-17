package webui

import (
	"strings"
	"testing"
)

func TestRenderSEOEscapesDynamicMetadata(t *testing.T) {
	source := []byte(`<html><head><title>old</title><meta name="description" content="old" /><link rel="canonical" href="" /><meta property="og:title" content="old" /><meta property="og:description" content="old" /><meta property="og:url" content="" /><meta property="og:image" content="" /><meta name="twitter:card" content="summary_large_image" /></head><body><div id="root"></div></body></html>`)
	result := string(renderSEO(source, SEO{
		Title:       `<unsafe>`,
		Description: `a "quote"`,
		Canonical:   `https://nav.ax/u/a?x=1&y=2`,
		Robots:      "index,follow",
		Image:       "https://nav.ax/og.jpg",
		SiteName:    "nav.ax",
		Locale:      "zh_CN",
		JSONLD:      `{"@type":"WebSite","name":"nav.ax"}`,
		Noscript:    `Hello <b>world</b>`,
	}))
	for _, expected := range []string{
		"&lt;unsafe&gt;",
		"&#34;quote&#34;",
		"x=1&amp;y=2",
		`name="robots" content="index,follow"`,
		`property="og:image" content="https://nav.ax/og.jpg"`,
		`name="twitter:image" content="https://nav.ax/og.jpg"`,
		`name="twitter:title" content="&lt;unsafe&gt;"`,
		`property="og:site_name" content="nav.ax"`,
		`property="og:locale" content="zh_CN"`,
		`application/ld+json`,
		`WebSite`,
		`<noscript><div>Hello &lt;b&gt;world&lt;/b&gt;</div></noscript>`,
	} {
		if !strings.Contains(result, expected) {
			t.Fatalf("rendered metadata missing %q: %s", expected, result)
		}
	}
}
