// Package webui serves the frontend embedded in the nav.ax binary.
package webui

import (
	"embed"
	"html"
	"io/fs"
	"net/http"
	"regexp"
	"strings"
)

//go:embed dist
var embedded embed.FS

type SEO struct {
	Title       string
	Description string
	Canonical   string
	Robots      string
	Image       string
}

type Options struct {
	ResolveSEO func(*http.Request) (SEO, error)
}

type Handler struct {
	files      fs.FS
	fileServer http.Handler
	index      []byte
	resolveSEO func(*http.Request) (SEO, error)
}

func New(options Options) (*Handler, error) {
	files, err := fs.Sub(embedded, "dist")
	if err != nil {
		return nil, err
	}
	index, _ := fs.ReadFile(files, "index.html")
	return &Handler{files: files, fileServer: http.FileServer(http.FS(files)), index: index, resolveSEO: options.ResolveSEO}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path != "" {
		if info, err := fs.Stat(h.files, path); err == nil && !info.IsDir() {
			if strings.HasPrefix(path, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			h.fileServer.ServeHTTP(w, r)
			return
		}
	}
	if len(h.index) == 0 {
		http.Error(w, "frontend assets are not embedded; run the production build", http.StatusServiceUnavailable)
		return
	}
	content := h.index
	if h.resolveSEO != nil {
		if seo, err := h.resolveSEO(r); err == nil {
			content = renderSEO(content, seo)
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

var (
	titlePattern       = regexp.MustCompile(`(?s)<title>.*?</title>`)
	descriptionPattern = regexp.MustCompile(`<meta name="description" content="[^"]*"\s*/?>`)
	canonicalPattern   = regexp.MustCompile(`<link rel="canonical" href="[^"]*"\s*/?>`)
	ogTitlePattern     = regexp.MustCompile(`<meta property="og:title" content="[^"]*"\s*/?>`)
	ogDescription      = regexp.MustCompile(`<meta property="og:description" content="[^"]*"\s*/?>`)
	ogURLPattern       = regexp.MustCompile(`<meta property="og:url" content="[^"]*"\s*/?>`)
	ogImagePattern     = regexp.MustCompile(`<meta property="og:image" content="[^"]*"\s*/?>`)
)

func renderSEO(source []byte, seo SEO) []byte {
	value := string(source)
	if seo.Title != "" {
		escaped := html.EscapeString(seo.Title)
		value = titlePattern.ReplaceAllString(value, "<title>"+escaped+"</title>")
		value = ogTitlePattern.ReplaceAllString(value, `<meta property="og:title" content="`+escaped+`" />`)
	}
	if seo.Description != "" {
		escaped := html.EscapeString(seo.Description)
		value = descriptionPattern.ReplaceAllString(value, `<meta name="description" content="`+escaped+`" />`)
		value = ogDescription.ReplaceAllString(value, `<meta property="og:description" content="`+escaped+`" />`)
	}
	if seo.Canonical != "" {
		escaped := html.EscapeString(seo.Canonical)
		value = canonicalPattern.ReplaceAllString(value, `<link rel="canonical" href="`+escaped+`" />`)
		value = ogURLPattern.ReplaceAllString(value, `<meta property="og:url" content="`+escaped+`" />`)
	}
	if seo.Image != "" {
		escaped := html.EscapeString(seo.Image)
		value = ogImagePattern.ReplaceAllString(value, `<meta property="og:image" content="`+escaped+`" />`)
	}
	if seo.Robots != "" {
		value = strings.Replace(value, "</head>", `    <meta name="robots" content="`+html.EscapeString(seo.Robots)+`" />`+"\n  </head>", 1)
	}
	return []byte(value)
}
