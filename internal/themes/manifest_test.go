package themes

import (
	"errors"
	"strings"
	"testing"
)

const minimalManifest = `{
  "specVersion": 1,
  "id": "sample",
  "name": "Sample",
  "version": "1.0.0",
  "author": "nav.ax",
  "mode": "light",
  "vibe": "serious",
  "swatches": ["#ffffff", "#888888", "#111111"],
  "tier": 1,
  "tokens": {
    "font": {"heading": "system-ui", "body": "system-ui", "label": "system-ui", "mono": "monospace"},
    "color": {
      "background": {"50": "0.99 0.003 12"},
      "foreground": {"900": "0.15 0.008 12"},
      "primary": {"500": "0.55 0.12 250"},
      "accent": {"500": "0.70 0.14 145"}
    }
  }
}`

func TestParseManifestAcceptsMinimal(t *testing.T) {
	m, err := ParseManifest([]byte(minimalManifest))
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}
	if m.ID != "sample" || m.Tier != 1 || m.Tokens.Font["body"] != "system-ui" {
		t.Fatalf("unexpected manifest: %+v", m)
	}
}

func TestParseManifestRejects(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(string) string
		wantMsg string
	}{
		{"未知 specVersion", func(s string) string { return strings.Replace(s, `"specVersion": 1`, `"specVersion": 2`, 1) }, "specVersion"},
		{"非法 id", func(s string) string { return strings.Replace(s, `"id": "sample"`, `"id": "Sample_1"`, 1) }, "id"},
		{"非法 mode", func(s string) string { return strings.Replace(s, `"mode": "light"`, `"mode": "neon"`, 1) }, "mode"},
		{"tier 越界", func(s string) string { return strings.Replace(s, `"tier": 1`, `"tier": 4`, 1) }, "tier"},
		{"tier 2 暂不支持", func(s string) string { return strings.Replace(s, `"tier": 1`, `"tier": 2`, 1) }, "tier"},
		{"色值非 OKLCH 三通道", func(s string) string {
			return strings.Replace(s, `"50": "0.99 0.003 12"`, `"50": "#ffffff"`, 1)
		}, "color"},
		{"色值超出 OKLCH 范围", func(s string) string {
			return strings.Replace(s, `"50": "0.99 0.003 12"`, `"50": "9 9 999"`, 1)
		}, "color"},
		{"缺必填字体族", func(s string) string {
			return strings.Replace(s, `"mono": "monospace"`, `"mono": ""`, 1)
		}, "font"},
		{"缺必填颜色组", func(s string) string {
			return strings.Replace(s, `"accent": {"500": "0.70 0.14 145"}`, `"accent": {}`, 1)
		}, "accent"},
		{"字体族含注入字符", func(s string) string {
			return strings.Replace(s, `"body": "system-ui"`, `"body": "a;}body{display:none"`, 1)
		}, "font"},
		{"swatch 非 hex", func(s string) string {
			return strings.Replace(s, `"#888888"`, `"rgb(1,2,3)"`, 1)
		}, "swatches"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseManifest([]byte(tc.mutate(minimalManifest)))
			if err == nil {
				t.Fatal("ParseManifest() expected error, got nil")
			}
			if !errors.Is(err, ErrInvalidManifest) {
				t.Fatalf("error = %v, want ErrInvalidManifest", err)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("error = %q, want to mention %q", err, tc.wantMsg)
			}
		})
	}
}

func TestParseManifestRejectsOversize(t *testing.T) {
	huge := `{"specVersion":1,"name":"` + strings.Repeat("a", maxManifestBytes) + `"}`
	if _, err := ParseManifest([]byte(huge)); err == nil {
		t.Fatal("ParseManifest() expected error for oversize manifest")
	}
}
