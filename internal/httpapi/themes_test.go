package httpapi_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/yixian-huang/navax/internal/database"
	"github.com/yixian-huang/navax/internal/httpapi"
	"github.com/yixian-huang/navax/internal/themes"
)

func newThemeTestServer(t *testing.T) (*sql.DB, *themes.Store, http.Handler) {
	t.Helper()
	db, err := database.OpenAndMigrate(context.Background(), database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatalf("OpenAndMigrate() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	store := themes.NewStore(db)
	router := chi.NewRouter()
	router.Route("/api/v1", func(api chi.Router) {
		httpapi.NewThemeHandler(store).MountPublic(api)
	})
	return db, store, router
}

// seedVersion 编译一个最小主题包并落库，返回版本 ID。
func seedVersion(t *testing.T, store *themes.Store) string {
	t.Helper()
	manifest, err := themes.ParseManifest([]byte(`{
	  "specVersion": 1, "id": "slate", "name": "Slate", "version": "1.0.0",
	  "author": "nav.ax", "mode": "light", "vibe": "serious",
	  "swatches": ["#ffffff", "#888888", "#111111"], "tier": 1,
	  "tokens": {
	    "font": {"heading": "system-ui", "body": "system-ui", "label": "system-ui", "mono": "monospace"},
	    "color": {
	      "background": {"50": "0.99 0.003 12"}, "foreground": {"900": "0.15 0.008 12"},
	      "primary": {"500": "0.55 0.12 250"}, "accent": {"500": "0.70 0.14 145"}
	    }
	  }
	}`))
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}
	png := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 32)...)
	asset, err := themes.ValidateAsset("img/noise.png", png)
	if err != nil {
		t.Fatalf("ValidateAsset() error = %v", err)
	}
	compiled, err := themes.Compile(themes.Package{
		Manifest: manifest,
		CSS:      []byte(`[data-nx="site-card"] { background-image: url("asset:img/noise.png"); }`),
		Assets:   []themes.Asset{asset},
	}, "slate")
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	versionID, err := store.UpsertVersion(context.Background(), "slate", compiled, "builtin", "builtin", time.Now().UTC())
	if err != nil {
		t.Fatalf("UpsertVersion() error = %v", err)
	}
	return versionID
}

func TestThemeCSSIsServedImmutably(t *testing.T) {
	_, store, router := newThemeTestServer(t)
	versionID := seedVersion(t, store)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/public/themes/"+versionID+".css", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/css; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if recorder.Header().Get("ETag") == "" {
		t.Fatal("ETag must be set so conditional requests work")
	}
	if !strings.Contains(recorder.Body.String(), `[data-theme="slate"]`) {
		t.Fatalf("body is not scoped compiled css:\n%s", recorder.Body.String())
	}
}

func TestThemeCSSHonoursIfNoneMatch(t *testing.T) {
	_, store, router := newThemeTestServer(t)
	versionID := seedVersion(t, store)

	first := httptest.NewRecorder()
	router.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/api/v1/public/themes/"+versionID+".css", nil))
	etag := first.Header().Get("ETag")

	request := httptest.NewRequest(http.MethodGet, "/api/v1/public/themes/"+versionID+".css", nil)
	request.Header.Set("If-None-Match", etag)
	second := httptest.NewRecorder()
	router.ServeHTTP(second, request)

	if second.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304", second.Code)
	}
}

func TestThemeCSSUnknownVersionIsNotFound(t *testing.T) {
	_, _, router := newThemeTestServer(t)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/public/themes/v00000000000000000000000000000000.css", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
}

// 撤销的版本要和「从未存在」区分开：410 而不是 404。
func TestThemeCSSDisabledVersionIsGone(t *testing.T) {
	db, store, router := newThemeTestServer(t)
	versionID := seedVersion(t, store)
	if _, err := db.Exec(`UPDATE themes SET current_version_id = NULL WHERE id = 'slate'`); err != nil {
		t.Fatalf("clear current version: %v", err)
	}
	if _, err := db.Exec(`UPDATE theme_versions SET status = ? WHERE id = ?`, themes.VersionStatusDisabled, versionID); err != nil {
		t.Fatalf("disable version: %v", err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/public/themes/"+versionID+".css", nil))
	if recorder.Code != http.StatusGone {
		t.Fatalf("status = %d, want 410", recorder.Code)
	}
}

func TestThemeAssetIsServed(t *testing.T) {
	_, store, router := newThemeTestServer(t)
	versionID := seedVersion(t, store)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/public/themes/"+versionID+"/assets/img/noise.png", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("Content-Type = %q, want image/png", got)
	}
	if got := recorder.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", got)
	}
}

// 资产按登记的 path 精确匹配，绝不用请求路径去拼任何真实路径。
func TestThemeAssetRejectsUnregisteredPaths(t *testing.T) {
	_, store, router := newThemeTestServer(t)
	versionID := seedVersion(t, store)

	for _, path := range []string{
		"img/absent.png",
		"../../../etc/passwd",
		"img/../img/noise.png",
	} {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/public/themes/"+versionID+"/assets/"+path, nil))
			if recorder.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", recorder.Code)
			}
		})
	}
}
