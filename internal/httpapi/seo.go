package httpapi

import (
	"net/http"

	"github.com/yixian-huang/navax/internal/catalog"
	"github.com/yixian-huang/navax/internal/seo"
)

// SEOHandler serves robots.txt and sitemap.xml outside the SPA shell.
type SEOHandler struct {
	catalog       *catalog.Service
	publicBaseURL string
}

func NewSEOHandler(catalogService *catalog.Service, publicBaseURL string) *SEOHandler {
	return &SEOHandler{catalog: catalogService, publicBaseURL: publicBaseURL}
}

func (h *SEOHandler) Robots(w http.ResponseWriter, r *http.Request) {
	discoverEnabled := true
	if h.catalog != nil {
		if enabled, err := h.catalog.DiscoverEnabled(r.Context()); err == nil {
			discoverEnabled = enabled
		}
	}
	body := seo.RobotsTxt(h.publicBaseURL, discoverEnabled)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}

func (h *SEOHandler) Sitemap(w http.ResponseWriter, r *http.Request) {
	discoverEnabled := true
	var public []seo.URLEntry
	if h.catalog != nil {
		if enabled, err := h.catalog.DiscoverEnabled(r.Context()); err == nil {
			discoverEnabled = enabled
		}
		if pages, err := h.catalog.SitemapPublicPages(r.Context()); err == nil {
			public = make([]seo.URLEntry, 0, len(pages))
			for _, page := range pages {
				public = append(public, seo.URLEntry{
					Loc:     "/u/" + page.Slug,
					LastMod: page.PublishedAt,
				})
			}
		}
	}
	entries := seo.BuildDefaultEntries(h.publicBaseURL, discoverEnabled, public)
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=1800")
	w.WriteHeader(http.StatusOK)
	_ = seo.WriteSitemapXML(w, entries)
}
