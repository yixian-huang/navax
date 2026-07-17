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

// SEO is server-injected document metadata for SPA shells.
type SEO struct {
	Title       string
	Description string
	Canonical   string
	Robots      string
	Image       string
	SiteName    string
	Locale      string
	// JSONLD is raw JSON for a single application/ld+json script (already valid JSON).
	JSONLD string
	// Noscript is plain-text (or simple HTML) injected into <noscript> for non-JS crawlers.
	Noscript string
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
	ogSiteNamePattern  = regexp.MustCompile(`<meta property="og:site_name" content="[^"]*"\s*/?>`)
	ogLocalePattern    = regexp.MustCompile(`<meta property="og:locale" content="[^"]*"\s*/?>`)
	twCardPattern      = regexp.MustCompile(`<meta name="twitter:card" content="[^"]*"\s*/?>`)
	twTitlePattern     = regexp.MustCompile(`<meta name="twitter:title" content="[^"]*"\s*/?>`)
	twDescription      = regexp.MustCompile(`<meta name="twitter:description" content="[^"]*"\s*/?>`)
	twImagePattern     = regexp.MustCompile(`<meta name="twitter:image" content="[^"]*"\s*/?>`)
	robotsPattern      = regexp.MustCompile(`<meta name="robots" content="[^"]*"\s*/?>`)
)

func renderSEO(source []byte, seo SEO) []byte {
	value := string(source)
	extra := make([]string, 0, 12)

	setOrCollect := func(pattern *regexp.Regexp, tag string, present bool) {
		if !present {
			return
		}
		if pattern.MatchString(value) {
			value = pattern.ReplaceAllString(value, tag)
			return
		}
		extra = append(extra, "    "+tag)
	}

	if seo.Title != "" {
		escaped := html.EscapeString(seo.Title)
		value = titlePattern.ReplaceAllString(value, "<title>"+escaped+"</title>")
		setOrCollect(ogTitlePattern, `<meta property="og:title" content="`+escaped+`" />`, true)
		setOrCollect(twTitlePattern, `<meta name="twitter:title" content="`+escaped+`" />`, true)
	}
	if seo.Description != "" {
		escaped := html.EscapeString(seo.Description)
		value = descriptionPattern.ReplaceAllString(value, `<meta name="description" content="`+escaped+`" />`)
		setOrCollect(ogDescription, `<meta property="og:description" content="`+escaped+`" />`, true)
		setOrCollect(twDescription, `<meta name="twitter:description" content="`+escaped+`" />`, true)
	}
	if seo.Canonical != "" {
		escaped := html.EscapeString(seo.Canonical)
		value = canonicalPattern.ReplaceAllString(value, `<link rel="canonical" href="`+escaped+`" />`)
		setOrCollect(ogURLPattern, `<meta property="og:url" content="`+escaped+`" />`, true)
	}
	if seo.Image != "" {
		escaped := html.EscapeString(seo.Image)
		setOrCollect(ogImagePattern, `<meta property="og:image" content="`+escaped+`" />`, true)
		setOrCollect(twImagePattern, `<meta name="twitter:image" content="`+escaped+`" />`, true)
	}
	if seo.SiteName != "" {
		escaped := html.EscapeString(seo.SiteName)
		setOrCollect(ogSiteNamePattern, `<meta property="og:site_name" content="`+escaped+`" />`, true)
	}
	if seo.Locale != "" {
		escaped := html.EscapeString(seo.Locale)
		setOrCollect(ogLocalePattern, `<meta property="og:locale" content="`+escaped+`" />`, true)
	}
	// Always prefer large image cards when we have an image; keep default card otherwise.
	if seo.Image != "" {
		setOrCollect(twCardPattern, `<meta name="twitter:card" content="summary_large_image" />`, true)
	}
	if seo.Robots != "" {
		escaped := html.EscapeString(seo.Robots)
		tag := `<meta name="robots" content="` + escaped + `" />`
		if robotsPattern.MatchString(value) {
			value = robotsPattern.ReplaceAllString(value, tag)
		} else {
			extra = append(extra, "    "+tag)
		}
	}
	if strings.TrimSpace(seo.JSONLD) != "" {
		// JSON-LD is trusted internal encoding; still strip closing script sequences.
		safe := strings.ReplaceAll(seo.JSONLD, "</", "<\\/")
		extra = append(extra, `    <script type="application/ld+json">`+safe+`</script>`)
	}

	if len(extra) > 0 {
		value = strings.Replace(value, "</head>", strings.Join(extra, "\n")+"\n  </head>", 1)
	}

	if strings.TrimSpace(seo.Noscript) != "" {
		// Escape noscript body as text so we never inject untrusted HTML.
		block := "<noscript><div>" + html.EscapeString(seo.Noscript) + "</div></noscript>"
		if strings.Contains(value, "<div id=\"root\"></div>") {
			value = strings.Replace(value, "<div id=\"root\"></div>", "<div id=\"root\"></div>\n    "+block, 1)
		} else {
			value = strings.Replace(value, "</body>", "    "+block+"\n  </body>", 1)
		}
	}
	return []byte(value)
}
