package navigation

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCleanSiteInputAcceptsLongMultibyteTitles(t *testing.T) {
	// 100 Chinese runes exceed 100 bytes; rune-based limits must accept them.
	title := strings.Repeat("测", 100)
	if utf8.RuneCountInString(title) != 100 || len(title) <= 100 {
		t.Fatalf("fixture should be 100 runes and >100 bytes, got runes=%d bytes=%d", utf8.RuneCountInString(title), len(title))
	}
	got, err := cleanSiteInput(SiteInput{
		CategoryID: "cat_test_category",
		Title:      title,
		URL:        "https://example.com/path",
	})
	if err != nil {
		t.Fatalf("cleanSiteInput() error = %v", err)
	}
	if got.Title != title {
		t.Fatalf("title was modified: %q", got.Title)
	}
}

func TestCleanSiteInputRejectsTitlesOver100Runes(t *testing.T) {
	_, err := cleanSiteInput(SiteInput{
		CategoryID: "cat_test_category",
		Title:      strings.Repeat("测", 101),
		URL:        "https://example.com/",
	})
	if err == nil {
		t.Fatal("expected validation error for 101-rune title")
	}
}

func TestCleanSiteInputRejectsDescriptionsOver300Runes(t *testing.T) {
	_, err := cleanSiteInput(SiteInput{
		CategoryID:  "cat_test_category",
		Title:       "ok",
		URL:         "https://example.com/",
		Description: strings.Repeat("描", 301),
	})
	if err == nil {
		t.Fatal("expected validation error for 301-rune description")
	}
}
