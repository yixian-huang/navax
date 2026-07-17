package navigation

import "testing"

func TestResolveOGImagePriority(t *testing.T) {
	poster := "/api/v1/assets/background/poster.jpg"
	page := Page{
		Publication: Publication{SEOImage: "/api/v1/assets/background/seo.jpg"},
		Settings: PageSettings{Appearance: AppearanceSettings{Background: BackgroundSettings{
			Type: "image", Value: "/api/v1/assets/background/bg.jpg", Poster: &poster,
		}}},
	}
	if got := resolveOGImage(page); got != "/api/v1/assets/background/seo.jpg" {
		t.Fatalf("prefer seo image, got %q", got)
	}

	page.Publication.SEOImage = ""
	if got := resolveOGImage(page); got != "/api/v1/assets/background/bg.jpg" {
		t.Fatalf("fallback image bg, got %q", got)
	}

	page.Settings.Appearance.Background.Type = "video"
	page.Settings.Appearance.Background.Value = "/api/v1/assets/background/v.mp4"
	if got := resolveOGImage(page); got != poster {
		t.Fatalf("fallback video poster, got %q", got)
	}

	page.Settings.Appearance.Background.Poster = nil
	if got := resolveOGImage(page); got != "" {
		t.Fatalf("empty without poster, got %q", got)
	}
}

func TestValidateSEOImage(t *testing.T) {
	if err := validateSEOImage(""); err != nil {
		t.Fatal(err)
	}
	if err := validateSEOImage("/api/v1/assets/background/a.jpg"); err != nil {
		t.Fatal(err)
	}
	if err := validateSEOImage("https://cdn.example.com/og.jpg"); err != nil {
		t.Fatal(err)
	}
	if err := validateSEOImage("javascript:alert(1)"); err == nil {
		t.Fatal("expected rejection")
	}
	if err := validateSEOImage("https://x.com/a b.jpg"); err == nil {
		t.Fatal("expected whitespace rejection")
	}
}
