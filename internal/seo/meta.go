// Package seo builds robots.txt, sitemap.xml, and document metadata for public routes.
package seo

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/yixian-huang/navax/internal/navigation"
	"github.com/yixian-huang/navax/internal/webui"
)

const (
	defaultDescription = "nav.ax 是一个开源、可自行部署的个性化导航站。收藏站点、创建分类、拖拽编排，打造属于你的互联网工作台。"
	defaultLocale      = "zh_CN"
)

// Config is the instance branding used when composing public metadata.
type Config struct {
	InstanceName  string
	PublicBaseURL string
}

func (c Config) base() string {
	return strings.TrimRight(strings.TrimSpace(c.PublicBaseURL), "/")
}

func (c Config) name() string {
	name := strings.TrimSpace(c.InstanceName)
	if name == "" {
		return "nav.ax"
	}
	return name
}

// AbsoluteURL joins PublicBaseURL with a path or returns absolute URLs unchanged.
func (c Config) AbsoluteURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	if !strings.HasPrefix(raw, "/") {
		raw = "/" + raw
	}
	return c.base() + raw
}

// StaticRoute returns SEO for non-page SPA routes (discover, auth, legal, app, …).
func (c Config) StaticRoute(pathname string) (webui.SEO, bool) {
	pathname = normalizePath(pathname)
	name := c.name()
	base := c.base()

	type routeMeta struct {
		title       string
		description string
		robots      string
		path        string
		indexable   bool
	}

	routes := map[string]routeMeta{
		"/discover": {
			title:       "发现精选导航 — " + name,
			description: "浏览 " + name + " 上公开分享的精选导航页，发现工具、设计与效率资源合集。",
			robots:      "index,follow",
			path:        "/discover",
			indexable:   true,
		},
		"/privacy": {
			title:       "隐私政策 — " + name,
			description: name + " 隐私政策：我们如何处理账号、会话与访问数据。",
			robots:      "index,follow",
			path:        "/privacy",
			indexable:   true,
		},
		"/terms": {
			title:       "服务条款 — " + name,
			description: name + " 服务条款与使用约定。",
			robots:      "index,follow",
			path:        "/terms",
			indexable:   true,
		},
		"/cookies": {
			title:       "Cookie 说明 — " + name,
			description: name + " Cookie 与本地存储使用说明。",
			robots:      "index,follow",
			path:        "/cookies",
			indexable:   true,
		},
		"/login": {
			title:       "登录 — " + name,
			description: "登录 " + name + " 管理你的私人导航。",
			robots:      "noindex,follow",
		},
		"/register": {
			title:       "注册 — " + name,
			description: "通过邀请注册 " + name + " 账号。",
			robots:      "noindex,follow",
		},
		"/invite": {
			title:  "邀请注册 — " + name,
			robots: "noindex,nofollow",
		},
		"/forgot-password": {
			title:  "找回密码 — " + name,
			robots: "noindex,nofollow",
		},
		"/reset-password": {
			title:  "重置密码 — " + name,
			robots: "noindex,nofollow",
		},
		"/setup": {
			title:  "初始化 — " + name,
			robots: "noindex,nofollow",
		},
	}

	if meta, ok := routes[pathname]; ok {
		canonical := ""
		if meta.indexable && meta.path != "" {
			canonical = base + meta.path
		}
		desc := meta.description
		if desc == "" {
			desc = defaultDescription
		}
		return c.shell(webui.SEO{
			Title:       meta.title,
			Description: desc,
			Canonical:   canonical,
			Robots:      meta.robots,
		}), true
	}

	// Workspace / admin surfaces should never be indexed.
	if strings.HasPrefix(pathname, "/app") || strings.HasPrefix(pathname, "/admin") {
		return c.shell(webui.SEO{
			Title:       "工作台 — " + name,
			Description: defaultDescription,
			Robots:      "noindex,nofollow",
		}), true
	}

	return webui.SEO{}, false
}

// FromPublishedPage builds SEO for system home or /u/{slug} published pages.
func (c Config) FromPublishedPage(page navigation.PublishedPage, requestPath string, host string) webui.SEO {
	name := c.name()
	base := c.base()
	requestPath = normalizePath(requestPath)

	title := strings.TrimSpace(page.SEOTitle)
	if title == "" {
		title = strings.TrimSpace(page.Title)
	}
	if page.Kind == navigation.PageKindSystem {
		title = strengthenSystemTitle(name, title)
	} else if title == "" {
		title = name
	}

	description := strings.TrimSpace(page.SEODescription)
	if description == "" {
		description = strings.TrimSpace(page.Description)
	}
	if description == "" {
		if page.Kind == navigation.PageKindSystem {
			description = defaultDescription
		} else {
			description = fmt.Sprintf("%s 的公开导航合集，由 %s 提供。", title, name)
		}
	}

	canonical := base + requestPath
	if requestPath == "/" {
		if page.Subdomain != nil && strings.TrimSpace(*page.Subdomain) != "" {
			scheme := "https://"
			if strings.HasPrefix(base, "http://") {
				scheme = "http://"
			}
			canonical = scheme + strings.TrimSuffix(strings.TrimSpace(*page.Subdomain), "/") + "/"
		} else {
			canonical = base + "/"
		}
	}

	robots := "noindex,follow"
	if page.Visibility == navigation.VisibilityPublic || page.Kind == navigation.PageKindSystem {
		robots = "index,follow"
	}

	image := c.AbsoluteURL(page.OGImage)
	seo := c.shell(webui.SEO{
		Title:       title,
		Description: description,
		Canonical:   canonical,
		Robots:      robots,
		Image:       image,
		Noscript:    noscriptFromPage(page, name),
	})

	if page.Kind == navigation.PageKindSystem && requestPath == "/" {
		seo.JSONLD = mustJSON(c.websiteJSONLD(title, description, canonical))
	} else if page.Visibility == navigation.VisibilityPublic {
		seo.JSONLD = mustJSON(map[string]any{
			"@context":    "https://schema.org",
			"@type":       "WebPage",
			"name":        title,
			"description": description,
			"url":         canonical,
			"isPartOf": map[string]any{
				"@type": "WebSite",
				"name":  name,
				"url":   base + "/",
			},
		})
	}
	_ = host
	return seo
}

func (c Config) shell(seo webui.SEO) webui.SEO {
	if seo.SiteName == "" {
		seo.SiteName = c.name()
	}
	if seo.Locale == "" {
		seo.Locale = defaultLocale
	}
	if seo.Description == "" {
		seo.Description = defaultDescription
	}
	return seo
}

func (c Config) websiteJSONLD(title, description, canonical string) map[string]any {
	name := c.name()
	base := c.base()
	return map[string]any{
		"@context": "https://schema.org",
		"@graph": []any{
			map[string]any{
				"@type":       "WebSite",
				"name":        name,
				"url":         base + "/",
				"description": description,
				"inLanguage":  "zh-CN",
			},
			map[string]any{
				"@type":               "WebApplication",
				"name":                name,
				"url":                 canonical,
				"applicationCategory": "BrowserApplication",
				"operatingSystem":     "Web",
				"description":         description,
				"offers": map[string]any{
					"@type":         "Offer",
					"price":         "0",
					"priceCurrency": "USD",
				},
			},
			map[string]any{
				"@type":       "WebPage",
				"name":        title,
				"description": description,
				"url":         canonical,
				"isPartOf": map[string]any{
					"@type": "WebSite",
					"name":  name,
					"url":   base + "/",
				},
			},
		},
	}
}

func strengthenSystemTitle(instanceName, title string) string {
	title = strings.TrimSpace(title)
	if title == "" || strings.EqualFold(title, instanceName) {
		return instanceName + " — 开源个性化导航站"
	}
	return title
}

func noscriptFromPage(page navigation.PublishedPage, instanceName string) string {
	var b strings.Builder
	title := strings.TrimSpace(page.Title)
	if title == "" {
		title = instanceName
	}
	b.WriteString(title)
	if desc := strings.TrimSpace(page.Description); desc != "" {
		b.WriteString(" — ")
		b.WriteString(desc)
	}
	b.WriteString("。")
	// List a short sample of sites so crawlers without JS still see content keywords.
	count := 0
	for _, cat := range page.Categories {
		for _, site := range cat.Sites {
			name := strings.TrimSpace(site.Title)
			if name == "" {
				continue
			}
			if count == 0 {
				b.WriteString(" 收录：")
			} else {
				b.WriteString("、")
			}
			b.WriteString(name)
			count++
			if count >= 24 {
				break
			}
		}
		if count >= 24 {
			break
		}
	}
	if count > 0 {
		b.WriteString("。")
	}
	b.WriteString(" 由 ")
	b.WriteString(instanceName)
	b.WriteString(" 提供。")
	return b.String()
}

func normalizePath(pathname string) string {
	pathname = strings.TrimSpace(pathname)
	if pathname == "" {
		return "/"
	}
	if u, err := url.Parse(pathname); err == nil && u.Path != "" {
		pathname = u.Path
	}
	if !strings.HasPrefix(pathname, "/") {
		pathname = "/" + pathname
	}
	if pathname != "/" {
		pathname = path.Clean(pathname)
	}
	return pathname
}

func mustJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(raw)
}
