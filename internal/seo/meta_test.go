package seo

import (
	"strings"
	"testing"
	"time"

	"github.com/yixian-huang/navax/internal/navigation"
)

func TestStaticRouteDiscover(t *testing.T) {
	cfg := Config{InstanceName: "nav.ax", PublicBaseURL: "https://nav.ax"}
	seo, ok := cfg.StaticRoute("/discover")
	if !ok {
		t.Fatal("expected discover route")
	}
	if seo.Canonical != "https://nav.ax/discover" {
		t.Fatalf("canonical = %q", seo.Canonical)
	}
	if seo.Robots != "index,follow" {
		t.Fatalf("robots = %q", seo.Robots)
	}
	if !strings.Contains(seo.Title, "发现") {
		t.Fatalf("title = %q", seo.Title)
	}
}

func TestStaticRouteAppNoindex(t *testing.T) {
	cfg := Config{InstanceName: "nav.ax", PublicBaseURL: "https://nav.ax"}
	seo, ok := cfg.StaticRoute("/app/links")
	if !ok {
		t.Fatal("expected app route")
	}
	if seo.Robots != "noindex,nofollow" {
		t.Fatalf("robots = %q", seo.Robots)
	}
	if seo.Canonical != "" {
		t.Fatalf("app pages should not set canonical, got %q", seo.Canonical)
	}
}

func TestStrengthenSystemTitle(t *testing.T) {
	if got := strengthenSystemTitle("nav.ax", "nav.ax"); got != "nav.ax — 开源个性化导航站" {
		t.Fatalf("got %q", got)
	}
	if got := strengthenSystemTitle("nav.ax", "我的导航工作台"); got != "我的导航工作台" {
		t.Fatalf("got %q", got)
	}
}

func TestFromPublishedPageSystem(t *testing.T) {
	cfg := Config{InstanceName: "nav.ax", PublicBaseURL: "https://nav.ax"}
	page := navigation.PublishedPage{
		Kind:        navigation.PageKindSystem,
		Title:       "nav.ax",
		SEOTitle:    "nav.ax",
		Visibility:  navigation.VisibilityPublic,
		OGImage:     "/api/v1/assets/background/x.jpg",
		Categories:  []navigation.PublicCategory{{Sites: []navigation.Site{{Title: "GitHub"}, {Title: "Figma"}}}},
		PublishedAt: time.Now(),
	}
	seo := cfg.FromPublishedPage(page, "/", "nav.ax")
	if seo.Title != "nav.ax — 开源个性化导航站" {
		t.Fatalf("title = %q", seo.Title)
	}
	if seo.Image != "https://nav.ax/api/v1/assets/background/x.jpg" {
		t.Fatalf("image = %q", seo.Image)
	}
	if seo.JSONLD == "" || !strings.Contains(seo.JSONLD, "WebSite") {
		t.Fatalf("jsonld = %q", seo.JSONLD)
	}
	if !strings.Contains(seo.Noscript, "GitHub") {
		t.Fatalf("noscript missing sites: %q", seo.Noscript)
	}
	if seo.SiteName != "nav.ax" || seo.Locale != "zh_CN" {
		t.Fatalf("site/locale = %q %q", seo.SiteName, seo.Locale)
	}
}

func TestRobotsAndSitemap(t *testing.T) {
	body := RobotsTxt("https://nav.ax", true)
	if !strings.Contains(body, "Sitemap: https://nav.ax/sitemap.xml") {
		t.Fatalf("robots missing sitemap: %s", body)
	}
	if !strings.Contains(body, "Disallow: /app/") {
		t.Fatalf("robots missing app disallow: %s", body)
	}
	entries := BuildDefaultEntries("https://nav.ax", true, []URLEntry{
		{Loc: "alice", LastMod: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)},
	})
	var buf strings.Builder
	if err := WriteSitemapXML(&buf, entries); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"https://nav.ax/",
		"https://nav.ax/discover",
		"https://nav.ax/u/alice",
		"<urlset",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("sitemap missing %q: %s", want, out)
		}
	}
}
