package backgrounds

import (
	"strings"
	"testing"
)

func TestClearBackgroundInSettingsJSON_ByMediaID(t *testing.T) {
	in := `{"appearance":{"themeId":"slate","background":{"type":"video","value":"/api/v1/assets/a.mp4","opacity":0.7,"mediaId":"bgm_abc","poster":"/p.jpg"}},"layout":{"template":"full"}}`
	out, changed := clearBackgroundInSettingsJSON(in, "/api/v1/assets/a.mp4", "bgm_abc")
	if !changed {
		t.Fatal("expected changed")
	}
	if strings.Contains(out, "bgm_abc") || strings.Contains(out, "a.mp4") {
		t.Fatalf("media reference should be cleared: %s", out)
	}
	if !strings.Contains(out, `"type":"none"`) {
		t.Fatalf("expected type none: %s", out)
	}
	if !strings.Contains(out, `"themeId":"slate"`) {
		t.Fatalf("themeId should be preserved: %s", out)
	}
}

func TestClearBackgroundInSettingsJSON_NoMatch(t *testing.T) {
	in := `{"appearance":{"background":{"type":"image","value":"https://example.com/x.jpg","opacity":1,"mediaId":"bgm_other"}}}`
	out, changed := clearBackgroundInSettingsJSON(in, "/api/v1/assets/a.mp4", "bgm_abc")
	if changed {
		t.Fatal("expected no change")
	}
	if out != in {
		t.Fatalf("settings mutated unexpectedly")
	}
}

func TestClearBackgroundInSettingsJSON_ByURLOnly(t *testing.T) {
	in := `{"appearance":{"background":{"type":"image","value":"https://cdn.example/bg.jpg","opacity":0.5}}}`
	out, changed := clearBackgroundInSettingsJSON(in, "https://cdn.example/bg.jpg", "bgm_unused")
	if !changed {
		t.Fatal("expected changed by URL")
	}
	if strings.Contains(out, "cdn.example") {
		t.Fatalf("url should be cleared: %s", out)
	}
}
