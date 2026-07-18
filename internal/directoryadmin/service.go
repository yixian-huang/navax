// Package directoryadmin implements privileged management of the public
// directory and cross-user personal navigation links.
package directoryadmin

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/yixian-huang/navax/internal/identity"
)

var (
	ErrForbidden     = errors.New("admin permission required")
	ErrInvalidInput  = errors.New("invalid input")
	ErrNotFound      = errors.New("not found")
	ErrConflict      = errors.New("conflict")
	ErrCategoryInUse = errors.New("directory category is not empty")
)

type Actor struct {
	ID       string
	Username string
	Role     string
	Status   string
}

type Page[T any] struct {
	Items    []T
	Page     int
	PageSize int
	Total    int
}

type Category struct {
	ID        string
	Name      string
	Icon      string
	SortOrder int
	Enabled   bool
	SiteCount int
}

type CategoryInput struct {
	Name    string
	Icon    string
	Enabled bool
}

type Site struct {
	ID           string
	CategoryID   string
	CategoryName string
	Title        string
	URL          string
	Icon         string
	Description  string
	SortOrder    int
	Enabled      bool
}

type SiteInput struct {
	CategoryID  string
	Title       string
	URL         string
	Icon        string
	Description string
	Enabled     bool
}

type SitePatch struct {
	CategoryID  *string
	Title       *string
	URL         *string
	Icon        *string
	Description *string
	Enabled     *bool
}

type SiteFilter struct {
	Search     string
	CategoryID string
	Page       int
	PageSize   int
}

type AdminLink struct {
	ID           string
	PageID       string
	CategoryID   string
	CategoryName string
	OwnerID      string
	OwnerName    string
	Title        string
	URL          string
	Icon         string
	Description  string
	SortOrder    int
	Enabled      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type LinkFilter struct {
	Search   string
	OwnerID  string
	Page     int
	PageSize int
}

type AuditRecord struct {
	ID         string
	ActorID    string
	ActorName  string
	Action     string
	TargetType string
	TargetID   string
	DetailJSON string
	RequestID  string
	CreatedAt  time.Time
}

type Store interface {
	Categories(context.Context) ([]Category, error)
	CreateCategory(context.Context, Category, time.Time, AuditRecord) (Category, error)
	UpdateCategory(context.Context, string, CategoryInput, time.Time, AuditRecord) (Category, error)
	DeleteCategory(context.Context, string, AuditRecord) error
	Sites(context.Context, SiteFilter) (Page[Site], error)
	CreateSite(context.Context, Site, time.Time, AuditRecord) (Site, error)
	UpdateSite(context.Context, string, SitePatch, time.Time, AuditRecord) (Site, error)
	DeleteSite(context.Context, string, AuditRecord) error
	Links(context.Context, LinkFilter) (Page[AdminLink], error)
	DeletePersonalLink(context.Context, string, time.Time, AuditRecord) error
}

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service { return &Service{store: store, now: time.Now} }

func (s *Service) Categories(ctx context.Context, actor Actor) ([]Category, error) {
	if err := authorize(actor); err != nil {
		return nil, err
	}
	return s.store.Categories(ctx)
}

func (s *Service) CreateCategory(ctx context.Context, actor Actor, input CategoryInput, requestID string) (Category, error) {
	if err := authorize(actor); err != nil {
		return Category{}, err
	}
	input.Name, input.Icon = strings.TrimSpace(input.Name), strings.TrimSpace(input.Icon)
	if runeLength(input.Name) < 1 || runeLength(input.Name) > 60 || runeLength(input.Icon) > 256 {
		return Category{}, ErrInvalidInput
	}
	id, err := identity.New("dct")
	if err != nil {
		return Category{}, err
	}
	now := s.now().UTC()
	audit, err := newAudit(actor, "directory.category.create", "directory_category", id, nil, requestID, now)
	if err != nil {
		return Category{}, err
	}
	return s.store.CreateCategory(ctx, Category{ID: id, Name: input.Name, Icon: input.Icon, Enabled: input.Enabled}, now, audit)
}

func (s *Service) UpdateCategory(ctx context.Context, actor Actor, categoryID string, input CategoryInput, requestID string) (Category, error) {
	if err := authorize(actor); err != nil {
		return Category{}, err
	}
	input.Name, input.Icon = strings.TrimSpace(input.Name), strings.TrimSpace(input.Icon)
	if categoryID == "" || runeLength(input.Name) < 1 || runeLength(input.Name) > 60 || runeLength(input.Icon) > 256 {
		return Category{}, ErrInvalidInput
	}
	now := s.now().UTC()
	audit, err := newAudit(actor, "directory.category.update", "directory_category", categoryID, nil, requestID, now)
	if err != nil {
		return Category{}, err
	}
	return s.store.UpdateCategory(ctx, categoryID, input, now, audit)
}

func (s *Service) DeleteCategory(ctx context.Context, actor Actor, categoryID, requestID string) error {
	if err := authorize(actor); err != nil {
		return err
	}
	if categoryID == "" {
		return ErrInvalidInput
	}
	audit, err := newAudit(actor, "directory.category.delete", "directory_category", categoryID, nil, requestID, s.now().UTC())
	if err != nil {
		return err
	}
	return s.store.DeleteCategory(ctx, categoryID, audit)
}

func (s *Service) Sites(ctx context.Context, actor Actor, filter SiteFilter) (Page[Site], error) {
	if err := authorize(actor); err != nil {
		return Page[Site]{}, err
	}
	filter.Page, filter.PageSize = pagination(filter.Page, filter.PageSize)
	filter.Search, filter.CategoryID = strings.TrimSpace(filter.Search), strings.TrimSpace(filter.CategoryID)
	return s.store.Sites(ctx, filter)
}

func (s *Service) CreateSite(ctx context.Context, actor Actor, input SiteInput, requestID string) (Site, error) {
	if err := authorize(actor); err != nil {
		return Site{}, err
	}
	clean, err := cleanSiteInput(input)
	if err != nil {
		return Site{}, err
	}
	id, err := identity.New("dst")
	if err != nil {
		return Site{}, err
	}
	now := s.now().UTC()
	audit, err := newAudit(actor, "directory.site.create", "directory_site", id, nil, requestID, now)
	if err != nil {
		return Site{}, err
	}
	return s.store.CreateSite(ctx, Site{
		ID: id, CategoryID: clean.CategoryID, Title: clean.Title, URL: clean.URL,
		Icon: clean.Icon, Description: clean.Description, Enabled: clean.Enabled,
	}, now, audit)
}

func (s *Service) UpdateSite(ctx context.Context, actor Actor, siteID string, patch SitePatch, requestID string) (Site, error) {
	if err := authorize(actor); err != nil {
		return Site{}, err
	}
	if siteID == "" {
		return Site{}, ErrInvalidInput
	}
	clean, err := cleanSitePatch(patch)
	if err != nil {
		return Site{}, err
	}
	now := s.now().UTC()
	audit, err := newAudit(actor, "directory.site.update", "directory_site", siteID, nil, requestID, now)
	if err != nil {
		return Site{}, err
	}
	return s.store.UpdateSite(ctx, siteID, clean, now, audit)
}

func (s *Service) DeleteSite(ctx context.Context, actor Actor, siteID, requestID string) error {
	if err := authorize(actor); err != nil {
		return err
	}
	if siteID == "" {
		return ErrInvalidInput
	}
	audit, err := newAudit(actor, "directory.site.delete", "directory_site", siteID, nil, requestID, s.now().UTC())
	if err != nil {
		return err
	}
	return s.store.DeleteSite(ctx, siteID, audit)
}

func (s *Service) Links(ctx context.Context, actor Actor, filter LinkFilter) (Page[AdminLink], error) {
	if err := authorize(actor); err != nil {
		return Page[AdminLink]{}, err
	}
	filter.Page, filter.PageSize = pagination(filter.Page, filter.PageSize)
	filter.Search, filter.OwnerID = strings.TrimSpace(filter.Search), strings.TrimSpace(filter.OwnerID)
	return s.store.Links(ctx, filter)
}

func (s *Service) DeleteLink(ctx context.Context, actor Actor, siteID, reason, requestID string) error {
	if err := authorize(actor); err != nil {
		return err
	}
	reason = strings.TrimSpace(reason)
	if siteID == "" || runeLength(reason) < 1 || runeLength(reason) > 300 {
		return ErrInvalidInput
	}
	now := s.now().UTC()
	audit, err := newAudit(actor, "link.admin_delete", "site", siteID, map[string]any{"reason": reason}, requestID, now)
	if err != nil {
		return err
	}
	return s.store.DeletePersonalLink(ctx, siteID, now, audit)
}

func authorize(actor Actor) error {
	if actor.ID == "" || actor.Username == "" || actor.Role != "admin" || actor.Status != "active" {
		return ErrForbidden
	}
	return nil
}

func cleanSiteInput(input SiteInput) (SiteInput, error) {
	input.CategoryID = strings.TrimSpace(input.CategoryID)
	input.Title, input.Icon = strings.TrimSpace(input.Title), strings.TrimSpace(input.Icon)
	input.Description = strings.TrimSpace(input.Description)
	if input.CategoryID == "" || runeLength(input.Title) < 1 || runeLength(input.Title) > 100 || runeLength(input.Icon) > 2048 || runeLength(input.Description) > 300 {
		return SiteInput{}, ErrInvalidInput
	}
	cleanURL, err := cleanHTTPURL(input.URL)
	if err != nil {
		return SiteInput{}, err
	}
	input.URL = cleanURL
	return input, nil
}

func cleanSitePatch(patch SitePatch) (SitePatch, error) {
	if patch.CategoryID == nil && patch.Title == nil && patch.URL == nil && patch.Icon == nil && patch.Description == nil && patch.Enabled == nil {
		return SitePatch{}, ErrInvalidInput
	}
	if patch.CategoryID != nil {
		value := strings.TrimSpace(*patch.CategoryID)
		if value == "" {
			return SitePatch{}, ErrInvalidInput
		}
		patch.CategoryID = &value
	}
	if patch.Title != nil {
		value := strings.TrimSpace(*patch.Title)
		if runeLength(value) < 1 || runeLength(value) > 100 {
			return SitePatch{}, ErrInvalidInput
		}
		patch.Title = &value
	}
	if patch.URL != nil {
		value, err := cleanHTTPURL(*patch.URL)
		if err != nil {
			return SitePatch{}, err
		}
		patch.URL = &value
	}
	if patch.Icon != nil {
		value := strings.TrimSpace(*patch.Icon)
		if runeLength(value) > 2048 {
			return SitePatch{}, ErrInvalidInput
		}
		patch.Icon = &value
	}
	if patch.Description != nil {
		value := strings.TrimSpace(*patch.Description)
		if runeLength(value) > 300 {
			return SitePatch{}, ErrInvalidInput
		}
		patch.Description = &value
	}
	return patch, nil
}

func cleanHTTPURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil || len(raw) > 2048 {
		return "", ErrInvalidInput
	}
	return parsed.String(), nil
}

func pagination(page, pageSize int) (int, int) {
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

func runeLength(value string) int { return len([]rune(value)) }

func newAudit(actor Actor, action, targetType, targetID string, detail any, requestID string, now time.Time) (AuditRecord, error) {
	if len(requestID) > 128 {
		return AuditRecord{}, ErrInvalidInput
	}
	id, err := identity.New("aud")
	if err != nil {
		return AuditRecord{}, err
	}
	encoded := []byte("{}")
	if detail != nil {
		encoded, err = json.Marshal(detail)
		if err != nil {
			return AuditRecord{}, err
		}
	}
	return AuditRecord{
		ID: id, ActorID: actor.ID, ActorName: actor.Username, Action: action,
		TargetType: targetType, TargetID: targetID, DetailJSON: string(encoded),
		RequestID: requestID, CreatedAt: now,
	}, nil
}
