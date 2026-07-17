package navigation

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/yixian-huang/navax/internal/identity"
)

type Store interface {
	CurrentPage(context.Context, Actor, PageKind) (Page, error)
	PageDraft(context.Context, Actor, string) (Page, error)
	UpdatePage(context.Context, Actor, string, PagePatch, time.Time) (Page, error)
	Categories(context.Context, Actor, string) ([]Category, error)
	CreateCategory(context.Context, Actor, string, Category, time.Time) (Category, error)
	UpdateCategory(context.Context, Actor, string, string, CategoryPatch, time.Time) (Category, error)
	DeleteCategory(context.Context, Actor, string, string, DeleteCategoryMode, time.Time) error
	Sites(context.Context, Actor, string, string, string) ([]Site, error)
	CreateSite(context.Context, Actor, string, Site, time.Time) (Site, error)
	UpdateSite(context.Context, Actor, string, string, SitePatch, time.Time) (Site, error)
	DeleteSite(context.Context, Actor, string, string, time.Time) error
	ReplaceContentOrder(context.Context, Actor, string, int, []CategoryOrder, time.Time) (int, error)
	Settings(context.Context, Actor, string) (PageSettings, error)
	ReplaceSettings(context.Context, Actor, string, int, PageSettings, time.Time) (PageSettings, error)
	Publication(context.Context, Actor, string, string) (Publication, error)
	ReplacePublication(context.Context, Actor, string, PublicationSettingsInput, string, time.Time) (Publication, error)
	Preview(context.Context, Actor, string, string, time.Time) (PublishedPage, error)
	Publish(context.Context, Actor, string, int, string, time.Time) (Publication, error)
	Unpublish(context.Context, Actor, string, string, time.Time) (Publication, error)
	PublicHome(context.Context) (PublishedPage, error)
	PublicHomeForHost(context.Context, string) (PublishedPage, error)
	PublicBySlug(context.Context, string) (PublishedPage, error)
}

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service { return &Service{store: store, now: time.Now} }

func (s *Service) CurrentPage(ctx context.Context, actor Actor, kind PageKind) (Page, error) {
	if kind != PageKindPersonal && kind != PageKindSystem {
		return Page{}, validation("kind", "must be personal or system")
	}
	return s.store.CurrentPage(ctx, actor, kind)
}

func (s *Service) PageDraft(ctx context.Context, actor Actor, pageID string) (Page, error) {
	return s.store.PageDraft(ctx, actor, pageID)
}

func (s *Service) UpdatePage(ctx context.Context, actor Actor, pageID string, patch PagePatch) (Page, error) {
	if patch.Title == nil && patch.Description == nil {
		return Page{}, validation("page", "at least one field is required")
	}
	if patch.Title != nil {
		v := strings.TrimSpace(*patch.Title)
		if len(v) < 1 || len(v) > 100 {
			return Page{}, validation("title", "length must be between 1 and 100")
		}
		patch.Title = &v
	}
	if patch.Description != nil {
		v := strings.TrimSpace(*patch.Description)
		if len(v) > 300 {
			return Page{}, validation("description", "length must not exceed 300")
		}
		patch.Description = &v
	}
	return s.store.UpdatePage(ctx, actor, pageID, patch, s.now().UTC())
}

func (s *Service) Categories(ctx context.Context, actor Actor, pageID string) ([]Category, error) {
	return s.store.Categories(ctx, actor, pageID)
}

func (s *Service) CreateCategory(ctx context.Context, actor Actor, pageID string, input CategoryInput) (Category, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Icon = strings.TrimSpace(input.Icon)
	if len(input.Name) < 1 || len(input.Name) > 60 {
		return Category{}, validation("name", "length must be between 1 and 60")
	}
	if len(input.Icon) > 256 {
		return Category{}, validation("icon", "length must not exceed 256")
	}
	id, err := identity.New("cat")
	if err != nil {
		return Category{}, err
	}
	return s.store.CreateCategory(ctx, actor, pageID, Category{ID: id, Name: input.Name, Icon: input.Icon}, s.now().UTC())
}

func (s *Service) UpdateCategory(ctx context.Context, actor Actor, pageID, categoryID string, patch CategoryPatch) (Category, error) {
	if patch.Name == nil && patch.Icon == nil {
		return Category{}, validation("category", "at least one field is required")
	}
	if patch.Name != nil {
		v := strings.TrimSpace(*patch.Name)
		if len(v) < 1 || len(v) > 60 {
			return Category{}, validation("name", "length must be between 1 and 60")
		}
		patch.Name = &v
	}
	if patch.Icon != nil {
		v := strings.TrimSpace(*patch.Icon)
		if len(v) > 256 {
			return Category{}, validation("icon", "length must not exceed 256")
		}
		patch.Icon = &v
	}
	return s.store.UpdateCategory(ctx, actor, pageID, categoryID, patch, s.now().UTC())
}

func (s *Service) DeleteCategory(ctx context.Context, actor Actor, pageID, categoryID string, mode DeleteCategoryMode) error {
	if mode != DeleteCategoryRejectIfNotEmpty && mode != DeleteCategoryDeleteSites && mode != DeleteCategoryMoveSites {
		return validation("mode", "unknown category deletion mode")
	}
	return s.store.DeleteCategory(ctx, actor, pageID, categoryID, mode, s.now().UTC())
}

func (s *Service) Sites(ctx context.Context, actor Actor, pageID, categoryID, search string) ([]Site, error) {
	return s.store.Sites(ctx, actor, pageID, categoryID, strings.TrimSpace(search))
}

func (s *Service) CreateSite(ctx context.Context, actor Actor, pageID string, input SiteInput) (Site, error) {
	clean, err := cleanSiteInput(input)
	if err != nil {
		return Site{}, err
	}
	id, err := identity.New("sit")
	if err != nil {
		return Site{}, err
	}
	return s.store.CreateSite(ctx, actor, pageID, Site{ID: id, CategoryID: clean.CategoryID, Title: clean.Title, URL: clean.URL, Icon: clean.Icon, Description: clean.Description}, s.now().UTC())
}

func (s *Service) UpdateSite(ctx context.Context, actor Actor, pageID, siteID string, patch SitePatch) (Site, error) {
	if patch.CategoryID == nil && patch.Title == nil && patch.URL == nil && patch.Icon == nil && patch.Description == nil {
		return Site{}, validation("site", "at least one field is required")
	}
	if patch.CategoryID != nil {
		v := strings.TrimSpace(*patch.CategoryID)
		if v == "" {
			return Site{}, validation("categoryId", "is required")
		}
		patch.CategoryID = &v
	}
	if patch.Title != nil {
		v := strings.TrimSpace(*patch.Title)
		if len(v) < 1 || len(v) > 100 {
			return Site{}, validation("title", "length must be between 1 and 100")
		}
		patch.Title = &v
	}
	if patch.URL != nil {
		v, err := cleanHTTPURL(*patch.URL)
		if err != nil {
			return Site{}, err
		}
		patch.URL = &v
	}
	if patch.Icon != nil {
		v := strings.TrimSpace(*patch.Icon)
		if len(v) > 2048 {
			return Site{}, validation("icon", "length must not exceed 2048")
		}
		patch.Icon = &v
	}
	if patch.Description != nil {
		v := strings.TrimSpace(*patch.Description)
		if len(v) > 300 {
			return Site{}, validation("description", "length must not exceed 300")
		}
		patch.Description = &v
	}
	return s.store.UpdateSite(ctx, actor, pageID, siteID, patch, s.now().UTC())
}

func (s *Service) DeleteSite(ctx context.Context, actor Actor, pageID, siteID string) error {
	return s.store.DeleteSite(ctx, actor, pageID, siteID, s.now().UTC())
}

func (s *Service) ReplaceContentOrder(ctx context.Context, actor Actor, pageID string, expectedRevision int, order []CategoryOrder) (int, error) {
	if len(order) > 50 {
		return 0, validation("categories", "must not exceed 50")
	}
	for _, category := range order {
		if len(category.SiteIDs) > 1000 {
			return 0, validation("siteIds", "must not exceed 1000")
		}
	}
	return s.store.ReplaceContentOrder(ctx, actor, pageID, expectedRevision, order, s.now().UTC())
}

func (s *Service) Settings(ctx context.Context, actor Actor, pageID string) (PageSettings, error) {
	return s.store.Settings(ctx, actor, pageID)
}

func (s *Service) ReplaceSettings(ctx context.Context, actor Actor, pageID string, expectedRevision int, settings PageSettings) (PageSettings, error) {
	if err := ValidateSettings(settings); err != nil {
		return PageSettings{}, err
	}
	return s.store.ReplaceSettings(ctx, actor, pageID, expectedRevision, settings, s.now().UTC())
}

func (s *Service) Publication(ctx context.Context, actor Actor, pageID, publicBaseURL string) (Publication, error) {
	return s.store.Publication(ctx, actor, pageID, strings.TrimRight(publicBaseURL, "/"))
}

func (s *Service) ReplacePublication(ctx context.Context, actor Actor, pageID string, input PublicationSettingsInput, publicBaseURL string) (Publication, error) {
	input.Slug = strings.ToLower(strings.TrimSpace(input.Slug))
	input.SEOTitle = strings.TrimSpace(input.SEOTitle)
	input.SEODescription = strings.TrimSpace(input.SEODescription)
	input.SEOImage = strings.TrimSpace(input.SEOImage)
	if input.Visibility != VisibilityPrivate && input.Visibility != VisibilityUnlisted && input.Visibility != VisibilityPublic {
		return Publication{}, validation("visibility", "must be private, unlisted, or public")
	}
	if !slugPattern.MatchString(input.Slug) {
		return Publication{}, validation("slug", "must contain 3-48 lowercase letters, digits, or internal hyphens")
	}
	if _, reserved := reservedSlugs[input.Slug]; reserved {
		return Publication{}, validation("slug", "is reserved by the system")
	}
	if len(input.SEOTitle) > 70 || len(input.SEODescription) > 160 {
		return Publication{}, validation("seo", "title or description is too long")
	}
	if err := validateSEOImage(input.SEOImage); err != nil {
		return Publication{}, err
	}
	return s.store.ReplacePublication(ctx, actor, pageID, input, strings.TrimRight(publicBaseURL, "/"), s.now().UTC())
}

func validateSEOImage(raw string) error {
	if raw == "" {
		return nil
	}
	if len(raw) > 2048 {
		return validation("seoImage", "must be at most 2048 characters")
	}
	// Same-origin asset paths or absolute http(s) URLs only.
	if strings.HasPrefix(raw, "/api/v1/assets/") {
		return nil
	}
	if strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "http://") {
		if strings.ContainsAny(raw, " \t\n\r") {
			return validation("seoImage", "must not contain whitespace")
		}
		return nil
	}
	return validation("seoImage", "must be an http(s) URL or /api/v1/assets/ path")
}

func (s *Service) Preview(ctx context.Context, actor Actor, pageID, publicBaseURL string) (PublishedPage, error) {
	return s.store.Preview(ctx, actor, pageID, strings.TrimRight(publicBaseURL, "/"), s.now().UTC())
}

func (s *Service) Publish(ctx context.Context, actor Actor, pageID string, expectedRevision int, publicBaseURL string) (Publication, error) {
	return s.store.Publish(ctx, actor, pageID, expectedRevision, strings.TrimRight(publicBaseURL, "/"), s.now().UTC())
}

func (s *Service) Unpublish(ctx context.Context, actor Actor, pageID, publicBaseURL string) (Publication, error) {
	return s.store.Unpublish(ctx, actor, pageID, strings.TrimRight(publicBaseURL, "/"), s.now().UTC())
}

func (s *Service) PublicHome(ctx context.Context) (PublishedPage, error) {
	return s.store.PublicHome(ctx)
}

func (s *Service) PublicHomeForHost(ctx context.Context, host string) (PublishedPage, error) {
	host = strings.ToLower(strings.Trim(strings.TrimSpace(host), "."))
	if host == "" {
		return s.store.PublicHome(ctx)
	}
	return s.store.PublicHomeForHost(ctx, host)
}

func (s *Service) PublicBySlug(ctx context.Context, slug string) (PublishedPage, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if !slugPattern.MatchString(slug) {
		return PublishedPage{}, ErrNotFound
	}
	return s.store.PublicBySlug(ctx, slug)
}

func ValidateSettings(settings PageSettings) error {
	if !oneOf(settings.Layout.Template, "full", "search-focus", "browse-first", "sidebar") {
		return validation("layout.template", "is invalid")
	}
	if !oneOf(settings.Layout.Density, "list", "compact", "comfortable") {
		return validation("layout.density", "is invalid")
	}
	if settings.Layout.Columns < 1 || settings.Layout.Columns > 8 || !oneOf(settings.Layout.CategoryStyle, "tabs", "sidebar", "grid") {
		return validation("layout", "columns or category style is invalid")
	}
	if strings.TrimSpace(settings.Appearance.ThemeID) == "" || !oneOf(settings.Appearance.Background.Type, "none", "color", "gradient", "image", "video") || len(settings.Appearance.Background.Value) > 2048 || settings.Appearance.Background.Opacity < 0 || settings.Appearance.Background.Opacity > 1 {
		return validation("appearance", "is invalid")
	}
	if !oneOf(settings.Search.DefaultEngine, "google", "bing", "duckduckgo", "baidu") {
		return validation("search.defaultEngine", "is invalid")
	}
	if strings.TrimSpace(settings.Preferences.Locale) == "" || len(settings.Preferences.Locale) > 20 || strings.TrimSpace(settings.Preferences.Timezone) == "" || len(settings.Preferences.Timezone) > 80 {
		return validation("preferences", "locale or timezone is invalid")
	}
	if _, err := time.LoadLocation(settings.Preferences.Timezone); err != nil {
		return validation("preferences.timezone", "is not a known timezone")
	}
	return nil
}

func cleanSiteInput(input SiteInput) (SiteInput, error) {
	input.CategoryID = strings.TrimSpace(input.CategoryID)
	input.Title = strings.TrimSpace(input.Title)
	input.Icon = strings.TrimSpace(input.Icon)
	input.Description = strings.TrimSpace(input.Description)
	if input.CategoryID == "" || len(input.Title) < 1 || len(input.Title) > 100 {
		return SiteInput{}, validation("site", "category and a title of at most 100 characters are required")
	}
	var err error
	input.URL, err = cleanHTTPURL(input.URL)
	if err != nil {
		return SiteInput{}, err
	}
	if len(input.Icon) > 2048 || len(input.Description) > 300 {
		return SiteInput{}, validation("site", "icon or description is too long")
	}
	return input, nil
}

func cleanHTTPURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return "", validation("url", "must be an absolute HTTP or HTTPS URL without credentials")
	}
	return u.String(), nil
}

func oneOf(value string, choices ...string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}

func validation(field, message string) error {
	return fmt.Errorf("%w: %s %s", ErrValidation, field, message)
}

var slugPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{1,46}[a-z0-9])$`)

var reservedSlugs = map[string]struct{}{
	"admin": {}, "api": {}, "app": {}, "assets": {}, "discover": {}, "favicon": {},
	"healthz": {}, "invite": {}, "login": {}, "nav": {}, "readyz": {}, "robots": {},
	"sitemap": {}, "u": {}, "www": {},
}
