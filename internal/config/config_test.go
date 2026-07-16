package config

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("NAVAX_DATA_DIR", t.TempDir())
	t.Setenv("NAVAX_SETUP_TOKEN", "01234567890123456789012345678901")
	t.Setenv("PUBLIC_BASE_URL", "http://localhost:8080/")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.PublicBaseURL != "http://localhost:8080" {
		t.Fatalf("PublicBaseURL = %q", cfg.PublicBaseURL)
	}
	if cfg.SecureCookies {
		t.Fatal("SecureCookies should default to false for HTTP")
	}
	if cfg.SessionTTL != 30*24*time.Hour {
		t.Fatalf("SessionTTL = %s", cfg.SessionTTL)
	}
	if len(cfg.MasterKey) != 32 {
		t.Fatalf("MasterKey length = %d", len(cfg.MasterKey))
	}
	if info, err := os.Stat(filepath.Join(cfg.DataDir, "master.key")); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("master key file = %v, %v", info, err)
	}
}

func TestLoadRejectsInvalidMasterKey(t *testing.T) {
	t.Setenv("NAVAX_SETUP_TOKEN", "01234567890123456789012345678901")
	t.Setenv("NAVAX_MASTER_KEY", base64.RawURLEncoding.EncodeToString([]byte("short")))

	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted an invalid master key")
	}
}
