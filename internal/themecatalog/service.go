// Package themecatalog owns catalog-promotion requests for private themes:
// an owner asks to publish a private theme into the official catalog, an
// admin approves or rejects. This is unrelated to internal/catalog (the
// site navigation directory/discover feed) — the shared "catalog" word
// here refers to themes.scope = 'catalog'.
package themecatalog

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/yixian-huang/navax/internal/identity"
)

var (
	ErrForbidden         = errors.New("admin permission required")
	ErrNotFound          = errors.New("theme catalog request not found")
	ErrConflict          = errors.New("theme catalog request conflicts with an active request")
	ErrInvalidTransition = errors.New("invalid theme catalog request status transition")
	ErrInvalidInput      = errors.New("invalid input")
	ErrSlugConflict      = errors.New("theme slug already exists in the catalog")
	ErrThemeNotEligible  = errors.New("theme is not eligible for catalog submission")
)

type Actor struct {
	ID       string
	Username string
	Role     string
	Status   string
}

type Request struct {
	ID         string
	ThemeID    string
	ThemeName  string
	Slug       string
	OwnerID    string
	OwnerName  string
	Status     string
	Reason     string
	VersionID  string
	ReviewerID string
	AppliedAt  time.Time
	ReviewedAt *time.Time
}

type Page struct {
	Items    []Request
	Page     int
	PageSize int
	Total    int
}

type CreateParams struct {
	Request
	Audit AuditRecord
}

type ReviewParams struct {
	RequestID  string
	ReviewerID string
	Decision   string
	Reason     string
	ReviewedAt time.Time
	Audit      AuditRecord
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
	Create(context.Context, CreateParams) (Request, error)
	CancelPending(context.Context, string, string, time.Time, AuditRecord) error
	List(context.Context, string, int, int) (Page, error)
	Review(context.Context, ReviewParams) (Request, error)
}

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service { return &Service{store: store, now: time.Now} }

// Request submits themeID (must be a private theme owned by actor) for
// catalog review. Store.Create does the DB-dependent eligibility checks
// (ownership, enabled, has a current version, slug not already in the
// catalog) because only a single transactional read can answer them
// consistently.
func (s *Service) Request(ctx context.Context, actor Actor, themeID, httpRequestID string) (Request, error) {
	if actor.ID == "" || actor.Username == "" {
		return Request{}, ErrInvalidInput
	}
	themeID = strings.TrimSpace(themeID)
	if themeID == "" {
		return Request{}, ErrInvalidInput
	}
	now := s.now().UTC()
	id, err := identity.New("tcr")
	if err != nil {
		return Request{}, err
	}
	audit, err := newAudit(actor.ID, actor.Username, "theme_catalog.apply", id, map[string]any{"themeId": themeID}, httpRequestID, now)
	if err != nil {
		return Request{}, err
	}
	return s.store.Create(ctx, CreateParams{
		Request: Request{ID: id, ThemeID: themeID, OwnerID: actor.ID, Status: "pending", AppliedAt: now},
		Audit:   audit,
	})
}

// Cancel lets the owner revoke their own pending request.
func (s *Service) Cancel(ctx context.Context, actor Actor, themeID, httpRequestID string) error {
	if actor.ID == "" || actor.Username == "" {
		return ErrInvalidInput
	}
	themeID = strings.TrimSpace(themeID)
	if themeID == "" {
		return ErrInvalidInput
	}
	now := s.now().UTC()
	audit, err := newAudit(actor.ID, actor.Username, "theme_catalog.revoke", themeID, nil, httpRequestID, now)
	if err != nil {
		return err
	}
	return s.store.CancelPending(ctx, actor.ID, themeID, now, audit)
}

func (s *Service) Requests(ctx context.Context, actor Actor, status string, page, pageSize int) (Page, error) {
	if err := authorize(actor); err != nil {
		return Page{}, err
	}
	if status != "" && status != "pending" && status != "approved" && status != "rejected" && status != "revoked" {
		return Page{}, ErrInvalidInput
	}
	page, pageSize = pagination(page, pageSize)
	return s.store.List(ctx, status, page, pageSize)
}

// Review is admin-only. decision is "approve" or "reject" — there is no
// "revoke" here: once a request is approved the theme is catalog-scoped,
// and taking it back down is the existing kill switch's job, not this
// state machine's.
func (s *Service) Review(ctx context.Context, actor Actor, requestID, decision, reason, httpRequestID string) (Request, error) {
	if err := authorize(actor); err != nil {
		return Request{}, err
	}
	reason = strings.TrimSpace(reason)
	if requestID == "" || (decision != "approve" && decision != "reject") || len([]rune(reason)) > 300 {
		return Request{}, ErrInvalidInput
	}
	now := s.now().UTC()
	audit, err := newAudit(actor.ID, actor.Username, "theme_catalog."+decision, requestID, map[string]any{"reason": reason}, httpRequestID, now)
	if err != nil {
		return Request{}, err
	}
	return s.store.Review(ctx, ReviewParams{
		RequestID: requestID, ReviewerID: actor.ID, Decision: decision,
		Reason: reason, ReviewedAt: now, Audit: audit,
	})
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

func newAudit(actorID, actorName, action, targetID string, detail any, requestID string, now time.Time) (AuditRecord, error) {
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
		ID: id, ActorID: actorID, ActorName: actorName, Action: action,
		TargetType: "theme_catalog_request", TargetID: targetID, DetailJSON: string(encoded),
		RequestID: requestID, CreatedAt: now,
	}, nil
}
