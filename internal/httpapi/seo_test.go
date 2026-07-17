package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/yixian-huang/navax/internal/catalog"
	"github.com/yixian-huang/navax/internal/database"
)

func TestSEORobotsAndSitemapRoutes(t *testing.T) {
	db, err := database.OpenAndMigrate(context.Background(), database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	catalogService := catalog.NewService(db)
	seoHandler := NewSEOHandler(catalogService, "https://nav.ax")
	handler := NewRouter(RouterOptions{
		PublicBaseURL: "https://nav.ax",
		MountRoot: func(router chi.Router) {
			router.Get("/robots.txt", seoHandler.Robots)
			router.Get("/sitemap.xml", seoHandler.Sitemap)
		},
		Web: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<html>spa</html>"))
		}),
	})

	req := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("robots status = %d body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/plain") {
		t.Fatalf("robots content-type = %q", ct)
	}
	if !strings.Contains(rec.Body.String(), "Sitemap: https://nav.ax/sitemap.xml") {
		t.Fatalf("robots body = %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "<html") {
		t.Fatal("robots must not fall through to SPA HTML")
	}

	req = httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("sitemap status = %d body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "xml") {
		t.Fatalf("sitemap content-type = %q", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "https://nav.ax/") || !strings.Contains(body, "<urlset") {
		t.Fatalf("sitemap body = %s", body)
	}
	if strings.Contains(body, "<html") {
		t.Fatal("sitemap must not fall through to SPA HTML")
	}
}
