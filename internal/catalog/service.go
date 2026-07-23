package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	adminpkg "github.com/yixian-huang/navax/internal/admin"
	"github.com/yixian-huang/navax/internal/navigation"
	"github.com/yixian-huang/navax/internal/themes"
)

// ErrDiscoverDisabled is returned when the instance has turned off the discover surface.
var ErrDiscoverDisabled = errors.New("discover is disabled")

type Config struct {
	InstanceName     string   `json:"instanceName"`
	PublicBaseURL    string   `json:"publicBaseUrl"`
	RootDomain       *string  `json:"rootDomain"`
	RegistrationMode string   `json:"registrationMode"`
	Features         Features `json:"features"`
	Limits           Limits   `json:"limits"`
}

type Features struct {
	Discover   bool `json:"discover"`
	Analytics  bool `json:"analytics"`
	Subdomains bool `json:"subdomains"`
	Mail       bool `json:"mail"`
}

type Limits struct {
	MaxCategoriesPerPage int   `json:"maxCategoriesPerPage"`
	MaxSitesPerPage      int   `json:"maxSitesPerPage"`
	MaxUploadBytes       int64 `json:"maxUploadBytes"`
}

type DirectorySite struct {
	ID           string `json:"id"`
	CategoryID   string `json:"categoryId"`
	CategoryName string `json:"categoryName"`
	Title        string `json:"title"`
	URL          string `json:"url"`
	Icon         string `json:"icon"`
	Description  string `json:"description"`
	SortOrder    int    `json:"sortOrder"`
	Enabled      bool   `json:"enabled"`
}

type DiscoverPage struct {
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description"`
	OwnerName   string `json:"ownerName"`
	ThemeID     string `json:"themeId"`
	// CoverImage is the share/wallpaper still used as the discover card hero.
	// Prefer dedicated OG image, then image background, then video poster.
	CoverImage  string    `json:"coverImage,omitempty"`
	Tags        []string  `json:"tags"`
	Featured    bool      `json:"featured"`
	ViewCount   int64     `json:"viewCount"`
	PublishedAt time.Time `json:"publishedAt"`
}

type Page[T any] struct {
	Items    []T
	Page     int
	PageSize int
	Total    int
}

type Service struct{ db *sql.DB }

func NewService(db *sql.DB) *Service { return &Service{db: db} }

func (s *Service) Config(ctx context.Context) (Config, error) {
	var config Config
	var rootDomain sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT instance_name, public_base_url, root_domain, registration_mode,
		       discover_enabled, analytics_enabled, subdomains_enabled,
		       max_categories_per_page, max_sites_per_page, max_upload_bytes
		FROM system_settings WHERE id = 1`,
	).Scan(
		&config.InstanceName, &config.PublicBaseURL, &rootDomain, &config.RegistrationMode,
		&config.Features.Discover, &config.Features.Analytics, &config.Features.Subdomains,
		&config.Limits.MaxCategoriesPerPage, &config.Limits.MaxSitesPerPage, &config.Limits.MaxUploadBytes,
	)
	if err != nil {
		return Config{}, err
	}
	if rootDomain.Valid {
		config.RootDomain = &rootDomain.String
	}
	// Password recovery depends on an enabled SMTP provider with settings.
	var mailEnabled bool
	var settingsJSON string
	if mailErr := s.db.QueryRowContext(ctx, `
		SELECT enabled, settings_json FROM provider_configs WHERE kind = 'smtp'`).Scan(&mailEnabled, &settingsJSON); mailErr == nil {
		config.Features.Mail = mailEnabled && len(strings.TrimSpace(settingsJSON)) > 2
	}
	return config, nil
}

// Themes 返回调用方可用的主题。谓词与预览、发布共用同一份定义
// （themes.EligibilityJoin/Where），各写一份 SQL 就会出现「列表里能选中、
// 发布时却静默回落」这种不一致。
//
// actorID 为空表示匿名调用：私有主题的 owner_id 非空，因此永不匹配。
func (s *Service) Themes(ctx context.Context, actorID string) ([]adminpkg.Theme, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT themes.id, themes.name, themes.version, themes.author, themes.description,
		       themes.mode, themes.preview, themes.enabled, themes.is_default,
		       themes.current_version_id, themes.scope,
		       theme_versions.manifest_json
		FROM themes `+themes.EligibilityJoin+`
		WHERE `+themes.EligibilityWhere+`
		ORDER BY themes.is_default DESC, themes.name, themes.id`, actorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]adminpkg.Theme, 0)
	for rows.Next() {
		var (
			theme        adminpkg.Theme
			manifestJSON string
		)
		if err := rows.Scan(&theme.ID, &theme.Name, &theme.Version, &theme.Author, &theme.Description,
			&theme.Mode, &theme.Preview, &theme.Enabled, &theme.Default,
			&theme.CurrentVersionID, &theme.Scope, &manifestJSON); err != nil {
			return nil, err
		}
		theme.CSSHref = "/api/v1/public/themes/" + theme.CurrentVersionID + ".css"
		var manifest themes.Manifest
		if err := json.Unmarshal([]byte(manifestJSON), &manifest); err != nil {
			return nil, err
		}
		theme.Subtitle = manifest.Subtitle
		theme.Tier = manifest.Tier
		theme.Vibe = manifest.Vibe
		theme.Swatches = manifest.Swatches
		list = append(list, theme)
	}
	return list, rows.Err()
}

func (s *Service) Directory(ctx context.Context, search, categoryID string, page, pageSize int) (Page[DirectorySite], error) {
	page, pageSize = normalizePagination(page, pageSize)
	search = strings.TrimSpace(search)
	where := " WHERE s.enabled = 1 AND c.enabled = 1"
	args := make([]any, 0, 6)
	if categoryID != "" {
		where += " AND s.category_id = ?"
		args = append(args, categoryID)
	}
	if search != "" {
		where += " AND (s.title LIKE ? ESCAPE '\\' OR s.description LIKE ? ESCAPE '\\' OR s.url LIKE ? ESCAPE '\\')"
		pattern := "%" + escapeLike(search) + "%"
		args = append(args, pattern, pattern, pattern)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM directory_sites s JOIN directory_categories c ON c.id = s.category_id"+where, args...).Scan(&total); err != nil {
		return Page[DirectorySite]{}, err
	}
	queryArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.id, s.category_id, c.name, s.title, s.url, s.icon, s.description, s.sort_order, s.enabled
		FROM directory_sites s JOIN directory_categories c ON c.id = s.category_id`+where+`
		ORDER BY c.sort_order, s.sort_order, s.id LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return Page[DirectorySite]{}, err
	}
	defer rows.Close()
	items := make([]DirectorySite, 0)
	for rows.Next() {
		var item DirectorySite
		if err := rows.Scan(&item.ID, &item.CategoryID, &item.CategoryName, &item.Title, &item.URL, &item.Icon, &item.Description, &item.SortOrder, &item.Enabled); err != nil {
			return Page[DirectorySite]{}, err
		}
		items = append(items, item)
	}
	return Page[DirectorySite]{Items: items, Page: page, PageSize: pageSize, Total: total}, rows.Err()
}

func (s *Service) Discover(ctx context.Context, search, tag, sort string, page, pageSize int) (Page[DiscoverPage], error) {
	var discoverEnabled bool
	if err := s.db.QueryRowContext(ctx, "SELECT discover_enabled FROM system_settings WHERE id = 1").Scan(&discoverEnabled); err != nil {
		return Page[DiscoverPage]{}, fmt.Errorf("read discover feature flag: %w", err)
	}
	if !discoverEnabled {
		return Page[DiscoverPage]{}, ErrDiscoverDisabled
	}

	page, pageSize = normalizePagination(page, pageSize)
	search, tag = strings.TrimSpace(search), strings.TrimSpace(tag)
	where := ` WHERE p.kind = 'personal' AND pp.visibility = 'public' AND pp.current_snapshot_id IS NOT NULL AND u.status = 'active'`
	args := make([]any, 0, 6)
	if search != "" {
		where += " AND (json_extract(s.payload_json, '$.title') LIKE ? ESCAPE '\\' OR json_extract(s.payload_json, '$.description') LIKE ? ESCAPE '\\')"
		pattern := "%" + escapeLike(search) + "%"
		args = append(args, pattern, pattern)
	}
	if tag != "" {
		where += " AND EXISTS (SELECT 1 FROM json_each(pp.tags_json) WHERE value = ?)"
		args = append(args, tag)
	}
	joins := ` FROM page_publications pp JOIN navigation_pages p ON p.id = pp.page_id JOIN users u ON u.id = p.owner_id JOIN published_snapshots s ON s.id = pp.current_snapshot_id`
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*)"+joins+where, args...).Scan(&total); err != nil {
		return Page[DiscoverPage]{}, err
	}
	order := "s.published_at DESC"
	switch sort {
	case "popular":
		order = "view_count DESC, s.published_at DESC"
	case "featured":
		order = "pp.featured DESC, s.published_at DESC"
	}
	queryArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	rows, err := s.db.QueryContext(ctx, `
		SELECT pp.slug, s.payload_json, pp.tags_json, pp.featured, s.published_at,
		       (SELECT COUNT(*) FROM analytics_events ae WHERE ae.page_id = p.id AND ae.event_type = 'page_view') AS view_count`+
		joins+where+" ORDER BY "+order+" LIMIT ? OFFSET ?", queryArgs...)
	if err != nil {
		return Page[DiscoverPage]{}, err
	}
	defer rows.Close()
	items := make([]DiscoverPage, 0)
	for rows.Next() {
		var item DiscoverPage
		var payloadJSON, tagsJSON, publishedAt string
		if err := rows.Scan(&item.Slug, &payloadJSON, &tagsJSON, &item.Featured, &publishedAt, &item.ViewCount); err != nil {
			return Page[DiscoverPage]{}, err
		}
		var published navigation.PublishedPage
		if err := json.Unmarshal([]byte(payloadJSON), &published); err != nil {
			return Page[DiscoverPage]{}, fmt.Errorf("decode published page: %w", err)
		}
		item.Title, item.Description, item.OwnerName = published.Title, published.Description, published.Owner.Name
		item.ThemeID = published.Settings.Appearance.ThemeID
		item.CoverImage = discoverCoverImage(published)
		if err := json.Unmarshal([]byte(tagsJSON), &item.Tags); err != nil {
			return Page[DiscoverPage]{}, err
		}
		item.PublishedAt, err = time.Parse(time.RFC3339Nano, publishedAt)
		if err != nil {
			return Page[DiscoverPage]{}, err
		}
		items = append(items, item)
	}
	return Page[DiscoverPage]{Items: items, Page: page, PageSize: pageSize, Total: total}, rows.Err()
}

// SitemapPublicPage is a minimal public page row for sitemap generation.
type SitemapPublicPage struct {
	Slug        string
	PublishedAt time.Time
}

// SitemapPublicPages lists personal public pages (slug + last publish time).
// Caps at 5000 entries to keep response size bounded.
func (s *Service) SitemapPublicPages(ctx context.Context) ([]SitemapPublicPage, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT pp.slug, s.published_at
		FROM page_publications pp
		JOIN navigation_pages p ON p.id = pp.page_id
		JOIN users u ON u.id = p.owner_id
		JOIN published_snapshots s ON s.id = pp.current_snapshot_id
		WHERE p.kind = 'personal'
		  AND pp.visibility = 'public'
		  AND pp.current_snapshot_id IS NOT NULL
		  AND u.status = 'active'
		ORDER BY s.published_at DESC
		LIMIT 5000`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]SitemapPublicPage, 0)
	for rows.Next() {
		var item SitemapPublicPage
		var publishedAt string
		if err := rows.Scan(&item.Slug, &publishedAt); err != nil {
			return nil, err
		}
		item.PublishedAt, err = time.Parse(time.RFC3339Nano, publishedAt)
		if err != nil {
			// Tolerate RFC3339 without nanos.
			if t2, err2 := time.Parse(time.RFC3339, publishedAt); err2 == nil {
				item.PublishedAt = t2
			}
		}
		if strings.TrimSpace(item.Slug) == "" {
			continue
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// DiscoverEnabled reports whether the public discover surface is on.
func (s *Service) DiscoverEnabled(ctx context.Context) (bool, error) {
	var enabled bool
	err := s.db.QueryRowContext(ctx, `SELECT discover_enabled FROM system_settings WHERE id = 1`).Scan(&enabled)
	return enabled, err
}

// discoverCoverImage picks a still frame suitable for discover card heroes.
func discoverCoverImage(page navigation.PublishedPage) string {
	if img := strings.TrimSpace(page.OGImage); img != "" {
		return img
	}
	bg := page.Settings.Appearance.Background
	switch strings.ToLower(strings.TrimSpace(bg.Type)) {
	case "image":
		return strings.TrimSpace(bg.Value)
	case "video":
		if bg.Poster != nil {
			return strings.TrimSpace(*bg.Poster)
		}
	}
	return ""
}

func normalizePagination(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func escapeLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}
