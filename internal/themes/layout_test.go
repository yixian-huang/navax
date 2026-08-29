package themes

import (
	"errors"
	"strings"
	"testing"
)

const validLayoutJSON = `{
  "template": { "default": "full", "allowed": ["full", "search-focus"], "locked": false },
  "density": { "default": "comfortable", "allowed": ["comfortable", "compact"], "locked": false },
  "columns": { "default": 4, "min": 2, "max": 6, "locked": false },
  "categoryStyle": { "default": "tabs", "allowed": ["tabs", "folders"], "locked": true },
  "sections": ["search", "greeting", "sites"]
}`

func withLayout(base string, layoutJSON string) string {
	return strings.Replace(base, `"tier": 1`, `"tier": 2, "layout": `+layoutJSON, 1)
}

func TestParseManifestAcceptsTier2Layout(t *testing.T) {
	m, err := ParseManifest([]byte(withLayout(minimalManifest, validLayoutJSON)))
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}
	if m.Tier != 2 || m.Layout == nil {
		t.Fatalf("expected tier 2 with layout, got %+v", m)
	}
	if m.Layout.Template.Default != "full" || m.Layout.Columns.Max != 6 {
		t.Fatalf("unexpected layout: %+v", m.Layout)
	}
}

func TestParseManifestRejectsTier1WithLayout(t *testing.T) {
	raw := strings.Replace(minimalManifest, `"tokens"`, `"layout": `+validLayoutJSON+`, "tokens"`, 1)
	_, err := ParseManifest([]byte(raw))
	if err == nil || !errors.Is(err, ErrInvalidManifest) || !strings.Contains(err.Error(), "layout") {
		t.Fatalf("error = %v, want layout rejection", err)
	}
}

func TestParseManifestRejectsInvalidLayout(t *testing.T) {
	tests := []struct {
		name   string
		layout string
		want   string
	}{
		{"allowed 空", `{
			"template": { "default": "full", "allowed": [], "locked": false },
			"density": { "default": "comfortable", "allowed": ["comfortable"], "locked": false },
			"columns": { "default": 4, "min": 1, "max": 8, "locked": false },
			"categoryStyle": { "default": "tabs", "allowed": ["tabs"], "locked": false }
		}`, "allowed"},
		{"default 不在 allowed", `{
			"template": { "default": "sidebar", "allowed": ["full"], "locked": false },
			"density": { "default": "comfortable", "allowed": ["comfortable"], "locked": false },
			"columns": { "default": 4, "min": 1, "max": 8, "locked": false },
			"categoryStyle": { "default": "tabs", "allowed": ["tabs"], "locked": false }
		}`, "default"},
		{"sections 缺 sites", `{
			"template": { "default": "full", "allowed": ["full"], "locked": false },
			"density": { "default": "comfortable", "allowed": ["comfortable"], "locked": false },
			"columns": { "default": 4, "min": 1, "max": 8, "locked": false },
			"categoryStyle": { "default": "tabs", "allowed": ["tabs"], "locked": false },
			"sections": ["greeting", "search"]
		}`, "sections"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseManifest([]byte(withLayout(minimalManifest, tc.layout)))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want to mention %q", err, tc.want)
			}
		})
	}
}

func TestApplyLayout(t *testing.T) {
	m, err := ParseManifest([]byte(withLayout(minimalManifest, validLayoutJSON)))
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}
	current := PageLayoutValues{Template: "sidebar", Density: "list", Columns: 8, CategoryStyle: "grid"}
	got := ApplyLayout(m.Layout, current)
	want := PageLayoutValues{Template: "full", Density: "comfortable", Columns: 6, CategoryStyle: "tabs"}
	if got != want {
		t.Fatalf("ApplyLayout() = %+v, want %+v", got, want)
	}

	inRange := PageLayoutValues{Template: "search-focus", Density: "compact", Columns: 3, CategoryStyle: "tabs"}
	kept := ApplyLayout(m.Layout, inRange)
	if kept.Template != "search-focus" || kept.Density != "compact" || kept.Columns != 3 {
		t.Fatalf("in-range values should stay, got %+v", kept)
	}
	if kept.CategoryStyle != "tabs" {
		t.Fatalf("locked categoryStyle = %q, want tabs", kept.CategoryStyle)
	}

	passthrough := PageLayoutValues{Template: "sidebar", Density: "list", Columns: 8, CategoryStyle: "grid"}
	if ApplyLayout(nil, passthrough) != passthrough {
		t.Fatal("nil spec must pass through")
	}
}
