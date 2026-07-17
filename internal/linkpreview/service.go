// Package linkpreview fetches public page metadata (title, description, icon)
// for the add-site UX. All fetches are SSRF-guarded via netguard.
package linkpreview

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"

	"github.com/yixian-huang/navax/internal/netguard"
)

const (
	maxBodyBytes   = 512 << 10 // 512 KiB is enough for <head>
	requestTimeout = 6 * time.Second
	maxRedirects   = 3
	userAgent      = "nav.ax-link-preview/1.0 (+https://nav.ax)"
)

var (
	ErrInvalidURL = errors.New("invalid url")
	ErrBlocked    = errors.New("url blocked")
	ErrFetch      = errors.New("fetch failed")
)

type Preview struct {
	URL         string  `json:"url"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	FaviconURL  string  `json:"faviconUrl"`
	SiteName    *string `json:"siteName,omitempty"`
}

type Service struct {
	client *http.Client
}

func NewService() *Service {
	validator := netguard.NewValidator(nil)
	return &Service{
		client: netguard.GuardedClient(validator, requestTimeout, maxRedirects),
	}
}

// Preview fetches metadata for a public HTTP(S) URL.
func (s *Service) Preview(ctx context.Context, raw string) (Preview, error) {
	normalized, err := normalizeURL(raw)
	if err != nil {
		return Preview{}, fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}
	parsed, err := url.Parse(normalized)
	if err != nil {
		return Preview{}, fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return Preview{}, fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.1")

	resp, err := s.client.Do(req)
	if err != nil {
		if errors.Is(err, netguard.ErrBlocked) || strings.Contains(err.Error(), "blocked") {
			return Preview{}, fmt.Errorf("%w: %v", ErrBlocked, err)
		}
		// Soft fallback: domain-only preview without remote HTML.
		return softPreview(parsed), nil
	}
	defer resp.Body.Close()

	finalURL := parsed
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return softPreview(finalURL), nil
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "" && !strings.Contains(strings.ToLower(contentType), "html") &&
		!strings.Contains(strings.ToLower(contentType), "xml") {
		return softPreview(finalURL), nil
	}

	limited := io.LimitReader(resp.Body, maxBodyBytes)
	reader, err := charset.NewReader(limited, contentType)
	if err != nil {
		reader = limited
	}
	doc, err := html.Parse(reader)
	if err != nil {
		return softPreview(finalURL), nil
	}

	meta := extractMeta(doc)
	title := firstNonEmpty(meta.ogTitle, meta.twitterTitle, meta.title)
	description := firstNonEmpty(meta.ogDescription, meta.twitterDescription, meta.description)
	icon := firstNonEmpty(meta.appleTouchIcon, meta.icon, meta.shortcutIcon, meta.ogImage)
	if icon != "" {
		icon = resolveURL(finalURL, icon)
	}
	if icon == "" {
		icon = googleFavicon(finalURL.Hostname())
	}

	title = sanitizeText(title, 200)
	description = sanitizeText(description, 500)
	if title == "" {
		title = domainTitle(finalURL.Hostname())
	}

	out := Preview{
		URL:         finalURL.String(),
		Title:       title,
		Description: description,
		FaviconURL:  icon,
	}
	if meta.ogSiteName != "" {
		name := sanitizeText(meta.ogSiteName, 100)
		out.SiteName = &name
	}
	return out, nil
}

func softPreview(u *url.URL) Preview {
	host := u.Hostname()
	return Preview{
		URL:        u.String(),
		Title:      domainTitle(host),
		FaviconURL: googleFavicon(host),
	}
}

func googleFavicon(host string) string {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "" {
		return ""
	}
	return "https://www.google.com/s2/favicons?domain=" + url.QueryEscape(host) + "&sz=64"
}

func domainTitle(host string) string {
	host = strings.TrimPrefix(strings.ToLower(host), "www.")
	if host == "" {
		return "站点"
	}
	part := strings.Split(host, ".")[0]
	if part == "" {
		return host
	}
	r, size := utf8.DecodeRuneInString(part)
	if r == utf8.RuneError {
		return part
	}
	return strings.ToUpper(string(r)) + part[size:]
}

func normalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("empty")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("only http/https")
	}
	if u.Host == "" {
		return "", errors.New("missing host")
	}
	return u.String(), nil
}

type pageMeta struct {
	title              string
	description        string
	ogTitle            string
	ogDescription      string
	ogImage            string
	ogSiteName         string
	twitterTitle       string
	twitterDescription string
	icon               string
	shortcutIcon       string
	appleTouchIcon     string
}

func extractMeta(n *html.Node) pageMeta {
	var m pageMeta
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			switch node.Data {
			case "title":
				if m.title == "" {
					m.title = textContent(node)
				}
			case "meta":
				name := strings.ToLower(attr(node, "name"))
				prop := strings.ToLower(attr(node, "property"))
				content := strings.TrimSpace(attr(node, "content"))
				if content == "" {
					break
				}
				switch {
				case name == "description":
					m.description = content
				case prop == "og:title":
					m.ogTitle = content
				case prop == "og:description":
					m.ogDescription = content
				case prop == "og:image":
					m.ogImage = content
				case prop == "og:site_name":
					m.ogSiteName = content
				case name == "twitter:title":
					m.twitterTitle = content
				case name == "twitter:description":
					m.twitterDescription = content
				}
			case "link":
				rel := strings.ToLower(attr(node, "rel"))
				href := strings.TrimSpace(attr(node, "href"))
				if href == "" {
					break
				}
				switch {
				case strings.Contains(rel, "apple-touch-icon"):
					if m.appleTouchIcon == "" {
						m.appleTouchIcon = href
					}
				case rel == "icon" || strings.Contains(rel, "icon") && !strings.Contains(rel, "mask"):
					// Prefer larger icons when sizes present; first wins for simplicity.
					if m.icon == "" {
						m.icon = href
					}
				case rel == "shortcut icon":
					if m.shortcutIcon == "" {
						m.shortcutIcon = href
					}
				}
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			// Stop deep body walks once we have title+icon — still cheap with 512KB cap.
			walk(c)
		}
	}
	walk(n)
	return m
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

func textContent(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(b.String())
}

func resolveURL(base *url.URL, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" || base == nil {
		return ref
	}
	u, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return base.ResolveReference(u).String()
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func sanitizeText(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if max > 0 && utf8.RuneCountInString(s) > max {
		runes := []rune(s)
		s = string(runes[:max])
	}
	return s
}
