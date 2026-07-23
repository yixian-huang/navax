// Package admin implements privileged account and instance administration.
// HTTP authentication is intentionally kept outside this package; every
// operation accepts an Actor and independently verifies the admin role.
package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/yixian-huang/navax/internal/auth"
	"github.com/yixian-huang/navax/internal/identity"
	"github.com/yixian-huang/navax/internal/security"
)

var (
	ErrForbidden       = errors.New("admin permission required")
	ErrInvalidInput    = errors.New("invalid input")
	ErrNotFound        = errors.New("not found")
	ErrConflict        = errors.New("conflict")
	ErrSelfDisable     = errors.New("an administrator cannot disable their own account")
	ErrDefaultTheme    = errors.New("the default theme must remain enabled")
	ErrInvitationState = errors.New("invitation is already revoked")
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

type UserFilter struct {
	Search   string
	Status   string
	Page     int
	PageSize int
}

type Health struct {
	Status        string
	UptimeSeconds int64
	Version       string
	GoVersion     string
	MemoryBytes   uint64
}

type Overview struct {
	TotalUsers        int
	ActiveUsers       int
	ActiveInvitations int
	PublicPages       int
	Health            Health
	RecentActions     []AuditEntry
}

type Invitation struct {
	ID           string
	TokenPreview string
	CreatorName  string
	Email        *string
	MaxUses      int
	UsedCount    int
	ExpiresAt    time.Time
	RevokedAt    *time.Time
	CreatedAt    time.Time
}

type InvitationCreate struct {
	Email         string
	MaxUses       int
	ExpiresInDays int
	SendEmail     bool
	PublicBaseURL string
	RequestID     string
}

type InvitationCreated struct {
	Invitation
	Token     string
	InviteURL string
}

type Theme struct {
	ID          string
	Name        string
	Subtitle    string
	Version     string
	Author      string
	Description string
	Mode        string
	Preview     string
	Enabled     bool
	Default     bool
	// 主题规范 v1 字段。CurrentVersionID 由编译产物的内容哈希派生，
	// CSSHref 据它拼出，两者共同让前端拿到可长缓存的样式表地址。
	CurrentVersionID string
	CSSHref          string
	Tier             int
	Scope            string
	Vibe             string
	Swatches         [3]string
}

type ThemePatch struct {
	Enabled   *bool
	Default   *bool
	RequestID string
}

type Limits struct {
	MaxCategoriesPerPage int
	MaxSitesPerPage      int
	MaxUploadBytes       int64
}

type AnalyticsSettings struct {
	Enabled       bool
	RetentionDays int
}

type DomainSettings struct {
	RootDomain        *string
	SubdomainsEnabled bool
}

type SystemSettings struct {
	InstanceName     string
	PublicBaseURL    string
	RegistrationMode string
	Limits           Limits
	Analytics        AnalyticsSettings
	Domain           DomainSettings
}

type LimitsPatch struct {
	MaxCategoriesPerPage *int
	MaxSitesPerPage      *int
	MaxUploadBytes       *int64
}

type AnalyticsPatch struct {
	Enabled       *bool
	RetentionDays *int
}

type DomainPatch struct {
	RootDomain        **string
	SubdomainsEnabled *bool
}

type SystemSettingsPatch struct {
	InstanceName     *string
	PublicBaseURL    *string
	RegistrationMode *string
	Limits           *LimitsPatch
	Analytics        *AnalyticsPatch
	Domain           *DomainPatch
	RequestID        string
}

type AuditEntry struct {
	ID         string
	ActorID    string
	ActorName  string
	Action     string
	TargetType string
	TargetID   string
	Detail     string
	RequestID  string
	CreatedAt  time.Time
}

type AuditFilter struct {
	Action   string
	Page     int
	PageSize int
}

type Counts struct {
	TotalUsers        int
	ActiveUsers       int
	ActiveInvitations int
	PublicPages       int
}

type InvitationInsert struct {
	Invitation
	TokenHash string
	CreatorID string
}

type AuditRecord struct {
	AuditEntry
}

// DiscoverPage is a public navigation page surface for admin curation.
type DiscoverPage struct {
	PageID      string    `json:"pageId"`
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	OwnerName   string    `json:"ownerName"`
	OwnerID     string    `json:"ownerId"`
	Featured    bool      `json:"featured"`
	Tags        []string  `json:"tags"`
	PublishedAt time.Time `json:"publishedAt"`
}

type DiscoverFilter struct {
	Search   string
	Page     int
	PageSize int
}

type DiscoverPatch struct {
	Featured *bool
	Tags     *[]string
}

type Store interface {
	OverviewCounts(context.Context, time.Time) (Counts, error)
	ListUsers(context.Context, UserFilter) (Page[auth.User], error)
	User(context.Context, string) (auth.User, error)
	SetUserStatus(context.Context, string, string, time.Time, AuditRecord) (auth.User, error)
	RevokeUserSessions(context.Context, string, time.Time, AuditRecord) error
	ListInvitations(context.Context, int, int) (Page[Invitation], error)
	InsertInvitation(context.Context, InvitationInsert, AuditRecord) error
	RevokeInvitation(context.Context, string, time.Time, AuditRecord) (Invitation, error)
	ListThemes(context.Context) ([]Theme, error)
	Theme(context.Context, string) (Theme, error)
	UpdateTheme(context.Context, string, ThemePatch, time.Time, AuditRecord) (Theme, error)
	Settings(context.Context) (SystemSettings, error)
	UpdateSettings(context.Context, SystemSettingsPatch, time.Time, AuditRecord) (SystemSettings, error)
	AppendAudit(context.Context, AuditRecord) error
	ListAudit(context.Context, AuditFilter) (Page[AuditEntry], error)
	ListDiscoverPages(context.Context, DiscoverFilter) (Page[DiscoverPage], error)
	UpdateDiscoverPage(context.Context, string, DiscoverPatch, time.Time, AuditRecord) (DiscoverPage, error)
}

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service { return &Service{store: store, now: time.Now} }

func (s *Service) Overview(ctx context.Context, actor Actor, health Health) (Overview, error) {
	if err := authorize(actor); err != nil {
		return Overview{}, err
	}
	counts, err := s.store.OverviewCounts(ctx, s.now().UTC())
	if err != nil {
		return Overview{}, err
	}
	recent, err := s.store.ListAudit(ctx, AuditFilter{Page: 1, PageSize: 10})
	if err != nil {
		return Overview{}, err
	}
	return Overview{counts.TotalUsers, counts.ActiveUsers, counts.ActiveInvitations, counts.PublicPages, health, recent.Items}, nil
}

func (s *Service) Users(ctx context.Context, actor Actor, filter UserFilter) (Page[auth.User], error) {
	if err := authorize(actor); err != nil {
		return Page[auth.User]{}, err
	}
	filter.Page, filter.PageSize = pagination(filter.Page, filter.PageSize)
	filter.Search = strings.TrimSpace(filter.Search)
	if filter.Status != "" && filter.Status != "active" && filter.Status != "disabled" {
		return Page[auth.User]{}, ErrInvalidInput
	}
	return s.store.ListUsers(ctx, filter)
}

func (s *Service) User(ctx context.Context, actor Actor, userID string) (auth.User, error) {
	if err := authorize(actor); err != nil {
		return auth.User{}, err
	}
	return s.store.User(ctx, userID)
}

func (s *Service) SetUserStatus(ctx context.Context, actor Actor, userID, status, reason, requestID string) (auth.User, error) {
	if err := authorize(actor); err != nil {
		return auth.User{}, err
	}
	if status != "active" && status != "disabled" || len([]rune(reason)) > 300 {
		return auth.User{}, ErrInvalidInput
	}
	if actor.ID == userID && status == "disabled" {
		return auth.User{}, ErrSelfDisable
	}
	audit, err := s.audit(actor, "user.status.update", "user", userID, map[string]any{"status": status, "reason": reason}, requestID)
	if err != nil {
		return auth.User{}, err
	}
	return s.store.SetUserStatus(ctx, userID, status, s.now().UTC(), audit)
}

func (s *Service) RevokeUserSessions(ctx context.Context, actor Actor, userID, requestID string) error {
	if err := authorize(actor); err != nil {
		return err
	}
	audit, err := s.audit(actor, "user.sessions.revoke", "user", userID, nil, requestID)
	if err != nil {
		return err
	}
	return s.store.RevokeUserSessions(ctx, userID, s.now().UTC(), audit)
}

func (s *Service) Invitations(ctx context.Context, actor Actor, page, pageSize int) (Page[Invitation], error) {
	if err := authorize(actor); err != nil {
		return Page[Invitation]{}, err
	}
	page, pageSize = pagination(page, pageSize)
	return s.store.ListInvitations(ctx, page, pageSize)
}

func (s *Service) CreateInvitation(ctx context.Context, actor Actor, input InvitationCreate) (InvitationCreated, error) {
	if err := authorize(actor); err != nil {
		return InvitationCreated{}, err
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if input.MaxUses < 1 || input.MaxUses > 100 || input.ExpiresInDays < 1 || input.ExpiresInDays > 365 {
		return InvitationCreated{}, ErrInvalidInput
	}
	if input.Email != "" {
		address, err := mail.ParseAddress(input.Email)
		if err != nil || !strings.EqualFold(address.Address, input.Email) {
			return InvitationCreated{}, ErrInvalidInput
		}
	}
	input.PublicBaseURL = strings.TrimRight(strings.TrimSpace(input.PublicBaseURL), "/")
	base, err := url.Parse(input.PublicBaseURL)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return InvitationCreated{}, ErrInvalidInput
	}
	if input.SendEmail && input.Email == "" {
		return InvitationCreated{}, ErrInvalidInput
	}
	id, err := identity.New("inv")
	if err != nil {
		return InvitationCreated{}, err
	}
	token, tokenHash, err := security.NewToken()
	if err != nil {
		return InvitationCreated{}, err
	}
	now := s.now().UTC()
	invitation := Invitation{ID: id, TokenPreview: token[:8] + "…", CreatorName: actor.Username, MaxUses: input.MaxUses, ExpiresAt: now.AddDate(0, 0, input.ExpiresInDays), CreatedAt: now}
	if input.Email != "" {
		invitation.Email = &input.Email
	}
	audit, err := s.audit(actor, "invitation.create", "invitation", id, map[string]any{"email": input.Email, "maxUses": input.MaxUses, "sendEmail": input.SendEmail}, input.RequestID)
	if err != nil {
		return InvitationCreated{}, err
	}
	if err := s.store.InsertInvitation(ctx, InvitationInsert{Invitation: invitation, TokenHash: tokenHash, CreatorID: actor.ID}, audit); err != nil {
		return InvitationCreated{}, err
	}
	// Path-style URL matches the SPA route /invite/:token and is copy-friendly.
	inviteURL := input.PublicBaseURL + "/invite/" + url.PathEscape(token)
	return InvitationCreated{Invitation: invitation, Token: token, InviteURL: inviteURL}, nil
}

func (s *Service) RevokeInvitation(ctx context.Context, actor Actor, invitationID, requestID string) (Invitation, error) {
	if err := authorize(actor); err != nil {
		return Invitation{}, err
	}
	audit, err := s.audit(actor, "invitation.revoke", "invitation", invitationID, nil, requestID)
	if err != nil {
		return Invitation{}, err
	}
	return s.store.RevokeInvitation(ctx, invitationID, s.now().UTC(), audit)
}

func (s *Service) Themes(ctx context.Context, actor Actor) ([]Theme, error) {
	if err := authorize(actor); err != nil {
		return nil, err
	}
	return s.store.ListThemes(ctx)
}

func (s *Service) UpdateTheme(ctx context.Context, actor Actor, themeID string, patch ThemePatch) (Theme, error) {
	if err := authorize(actor); err != nil {
		return Theme{}, err
	}
	if patch.Enabled == nil && patch.Default == nil {
		return Theme{}, ErrInvalidInput
	}
	if patch.Default != nil && *patch.Default && patch.Enabled != nil && !*patch.Enabled {
		return Theme{}, ErrDefaultTheme
	}
	current, err := s.store.Theme(ctx, themeID)
	if err != nil {
		return Theme{}, err
	}
	if current.Default && patch.Enabled != nil && !*patch.Enabled && (patch.Default == nil || *patch.Default) {
		return Theme{}, ErrDefaultTheme
	}
	audit, err := s.audit(actor, "theme.update", "theme", themeID, map[string]any{"enabled": patch.Enabled, "default": patch.Default}, patch.RequestID)
	if err != nil {
		return Theme{}, err
	}
	return s.store.UpdateTheme(ctx, themeID, patch, s.now().UTC(), audit)
}

func (s *Service) Settings(ctx context.Context, actor Actor) (SystemSettings, error) {
	if err := authorize(actor); err != nil {
		return SystemSettings{}, err
	}
	return s.store.Settings(ctx)
}

func (s *Service) UpdateSettings(ctx context.Context, actor Actor, patch SystemSettingsPatch) (SystemSettings, error) {
	if err := authorize(actor); err != nil {
		return SystemSettings{}, err
	}
	if err := validateSettingsPatch(patch); err != nil {
		return SystemSettings{}, err
	}
	// Enabling subdomains requires a non-empty root domain. If the admin toggles
	// the flag without setting one, derive the host from the public base URL.
	if patch.Domain != nil && patch.Domain.SubdomainsEnabled != nil && *patch.Domain.SubdomainsEnabled {
		needsRoot := patch.Domain.RootDomain == nil || *patch.Domain.RootDomain == nil || strings.TrimSpace(**patch.Domain.RootDomain) == ""
		if needsRoot {
			current, err := s.store.Settings(ctx)
			if err != nil {
				return SystemSettings{}, err
			}
			baseURL := current.PublicBaseURL
			if patch.PublicBaseURL != nil {
				baseURL = *patch.PublicBaseURL
			}
			if host := rootDomainFromPublicURL(baseURL); host != "" {
				root := host
				rootPtr := &root
				if patch.Domain.RootDomain == nil {
					patch.Domain.RootDomain = &rootPtr
				} else {
					*patch.Domain.RootDomain = rootPtr
				}
			} else {
				return SystemSettings{}, ErrInvalidInput
			}
		}
	}
	audit, err := s.audit(actor, "settings.update", "system_settings", "1", nil, patch.RequestID)
	if err != nil {
		return SystemSettings{}, err
	}
	return s.store.UpdateSettings(ctx, patch, s.now().UTC(), audit)
}

func rootDomainFromPublicURL(publicBaseURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(publicBaseURL))
	if err != nil || parsed.Host == "" {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	host = strings.TrimPrefix(host, "www.")
	if host == "" || host == "localhost" || strings.HasPrefix(host, "127.") {
		return ""
	}
	return host
}

func (s *Service) WriteAudit(ctx context.Context, actor Actor, action, targetType, targetID string, detail any, requestID string) error {
	if err := authorize(actor); err != nil {
		return err
	}
	record, err := s.audit(actor, action, targetType, targetID, detail, requestID)
	if err != nil {
		return err
	}
	return s.store.AppendAudit(ctx, record)
}

func (s *Service) DiscoverPages(ctx context.Context, actor Actor, filter DiscoverFilter) (Page[DiscoverPage], error) {
	if err := authorize(actor); err != nil {
		return Page[DiscoverPage]{}, err
	}
	filter.Page, filter.PageSize = pagination(filter.Page, filter.PageSize)
	filter.Search = strings.TrimSpace(filter.Search)
	return s.store.ListDiscoverPages(ctx, filter)
}

func (s *Service) UpdateDiscoverPage(ctx context.Context, actor Actor, pageID string, patch DiscoverPatch, requestID string) (DiscoverPage, error) {
	if err := authorize(actor); err != nil {
		return DiscoverPage{}, err
	}
	if patch.Featured == nil && patch.Tags == nil {
		return DiscoverPage{}, ErrInvalidInput
	}
	if patch.Tags != nil {
		cleaned := make([]string, 0, len(*patch.Tags))
		seen := map[string]struct{}{}
		for _, tag := range *patch.Tags {
			tag = strings.TrimSpace(tag)
			if tag == "" || len([]rune(tag)) > 32 {
				return DiscoverPage{}, ErrInvalidInput
			}
			key := strings.ToLower(tag)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			cleaned = append(cleaned, tag)
			if len(cleaned) > 12 {
				return DiscoverPage{}, ErrInvalidInput
			}
		}
		*patch.Tags = cleaned
	}
	audit, err := s.audit(actor, "discover.update", "page", pageID, map[string]any{
		"featured": patch.Featured, "tags": patch.Tags,
	}, requestID)
	if err != nil {
		return DiscoverPage{}, err
	}
	return s.store.UpdateDiscoverPage(ctx, pageID, patch, s.now().UTC(), audit)
}

func (s *Service) Audit(ctx context.Context, actor Actor, filter AuditFilter) (Page[AuditEntry], error) {
	if err := authorize(actor); err != nil {
		return Page[AuditEntry]{}, err
	}
	filter.Page, filter.PageSize = pagination(filter.Page, filter.PageSize)
	filter.Action = strings.TrimSpace(filter.Action)
	if len(filter.Action) > 80 {
		return Page[AuditEntry]{}, ErrInvalidInput
	}
	return s.store.ListAudit(ctx, filter)
}

func (s *Service) audit(actor Actor, action, targetType, targetID string, detail any, requestID string) (AuditRecord, error) {
	if action == "" || targetType == "" || len(action) > 100 || len(targetType) > 100 || len(targetID) > 128 || len(requestID) > 128 {
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
			return AuditRecord{}, fmt.Errorf("encode audit detail: %w", err)
		}
	}
	return AuditRecord{AuditEntry: AuditEntry{ID: id, ActorID: actor.ID, ActorName: actor.Username, Action: action, TargetType: targetType, TargetID: targetID, Detail: string(encoded), RequestID: requestID, CreatedAt: s.now().UTC()}}, nil
}

func authorize(actor Actor) error {
	if actor.ID == "" || actor.Username == "" || actor.Role != "admin" || actor.Status != "active" {
		return ErrForbidden
	}
	return nil
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

func validateSettingsPatch(patch SystemSettingsPatch) error {
	if patch.InstanceName == nil && patch.PublicBaseURL == nil && patch.RegistrationMode == nil && patch.Limits == nil && patch.Analytics == nil && patch.Domain == nil {
		return ErrInvalidInput
	}
	if patch.InstanceName != nil {
		value := strings.TrimSpace(*patch.InstanceName)
		if len([]rune(value)) < 1 || len([]rune(value)) > 60 {
			return ErrInvalidInput
		}
		*patch.InstanceName = value
	}
	if patch.PublicBaseURL != nil {
		value := strings.TrimRight(strings.TrimSpace(*patch.PublicBaseURL), "/")
		parsed, err := url.Parse(value)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || len(value) > 2048 {
			return ErrInvalidInput
		}
		*patch.PublicBaseURL = value
	}
	if patch.RegistrationMode != nil && *patch.RegistrationMode != "invite" && *patch.RegistrationMode != "closed" && *patch.RegistrationMode != "open" {
		return ErrInvalidInput
	}
	if patch.Limits != nil {
		limits := patch.Limits
		if limits.MaxCategoriesPerPage != nil && (*limits.MaxCategoriesPerPage < 1 || *limits.MaxCategoriesPerPage > 500) ||
			limits.MaxSitesPerPage != nil && (*limits.MaxSitesPerPage < 1 || *limits.MaxSitesPerPage > 10000) ||
			limits.MaxUploadBytes != nil && (*limits.MaxUploadBytes < 1024 || *limits.MaxUploadBytes > 52428800) {
			return ErrInvalidInput
		}
	}
	if patch.Analytics != nil && patch.Analytics.RetentionDays != nil && (*patch.Analytics.RetentionDays < 7 || *patch.Analytics.RetentionDays > 365) {
		return ErrInvalidInput
	}
	if patch.Domain != nil && patch.Domain.RootDomain != nil && *patch.Domain.RootDomain != nil {
		value := strings.ToLower(strings.TrimSpace(**patch.Domain.RootDomain))
		if value == "" || len(value) > 253 || strings.ContainsAny(value, "/: ") {
			return ErrInvalidInput
		}
		**patch.Domain.RootDomain = value
	}
	return nil
}
