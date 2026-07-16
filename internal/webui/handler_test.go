package webui

import (
	"strings"
	"testing"
)

func TestRenderSEOEscapesDynamicMetadata(t *testing.T) {
	source := []byte(`<html><head><title>old</title><meta name="description" content="old" /><link rel="canonical" href="" /><meta property="og:title" content="old" /><meta property="og:description" content="old" /><meta property="og:url" content="" /></head></html>`)
	result := string(renderSEO(source, SEO{
		Title: `<unsafe>`, Description: `a "quote"`, Canonical: `https://nav.ax/u/a?x=1&y=2`, Robots: "index,follow",
	}))
	for _, expected := range []string{"&lt;unsafe&gt;", "&#34;quote&#34;", "x=1&amp;y=2", `name="robots" content="index,follow"`} {
		if !strings.Contains(result, expected) {
			t.Fatalf("rendered metadata missing %q: %s", expected, result)
		}
	}
}
