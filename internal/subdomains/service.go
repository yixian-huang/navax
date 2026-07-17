// Package subdomains owns user subdomain requests and their admin review
// state machine. DNS provisioning is intentionally outside this package: a
// wildcard DNS/TLS deployment only needs the approved database mapping.
package subdomains

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/yixian-huang/navax/internal/identity"
)

var (
	ErrForbidden         = errors.New("admin permission required")
	ErrUnavailable       = errors.New("subdomains are not enabled")
	ErrInvalidLabel      = errors.New("invalid subdomain label")
	ErrReservedLabel     = errors.New("reserved subdomain label")
	ErrConflict          = errors.New("subdomain request conflicts with an active request")
	ErrNotFound          = errors.New("subdomain request not found")
	ErrInvalidTransition = errors.New("invalid subdomain status transition")
	ErrInvalidInput      = errors.New("invalid input")
)

const automaticApprovalMinLength = 4

var labelPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,28}[a-z0-9])?$`)

var reservedLabels = map[string]struct{}{
	"admin": {}, "api": {}, "app": {}, "assets": {}, "auth": {}, "cdn": {},
	"discover": {}, "invite": {}, "login": {}, "mail": {}, "nav": {}, "static": {},
	"status": {}, "support": {}, "u": {}, "www": {},
}

type Actor struct {
	ID       string
	Username string
	Role     string
	Status   string
}

type Request struct {
	ID           string
	UserID       string
	Username     string
	Label        string
	FullDomain   string
	CustomDomain *string
	Status       string
	AppliedAt    time.Time
	ReviewedAt   *time.Time
	Reason       string
}

type Page struct {
	Items    []Request
	Page     int
	PageSize int
	Total    int
}

type Policy struct {
	Enabled    bool
	RootDomain string
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
	Policy(context.Context) (Policy, error)
	LatestForUser(context.Context, string) (*Request, error)
	Create(context.Context, CreateParams) error
	CancelPending(context.Context, string, time.Time, AuditRecord) error
	List(context.Context, string, int, int) (Page, error)
	Review(context.Context, ReviewParams) (Request, error)
	SetCustomDomain(context.Context, string, *string, time.Time, AuditRecord) (Request, error)
}

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service { return &Service{store: store, now: time.Now} }

func (s *Service) Mine(ctx context.Context, userID string) (*Request, error) {
	if userID == "" {
		return nil, ErrInvalidInput
	}
	return s.store.LatestForUser(ctx, userID)
}

func (s *Service) Apply(ctx context.Context, userID, username, label, requestID string) (Request, error) {
	if userID == "" || username == "" {
		return Request{}, ErrInvalidInput
	}
	label = strings.TrimSpace(label)
	if !labelPattern.MatchString(label) {
		return Request{}, ErrInvalidLabel
	}
	if _, reserved := reservedLabels[label]; reserved {
		return Request{}, ErrReservedLabel
	}
	policy, err := s.store.Policy(ctx)
	if err != nil {
		return Request{}, err
	}
	rootDomain := strings.ToLower(strings.Trim(strings.TrimSpace(policy.RootDomain), "."))
	if !policy.Enabled || rootDomain == "" {
		return Request{}, ErrUnavailable
	}
	if len(label)+1+len(rootDomain) > 253 {
		return Request{}, ErrUnavailable
	}
	now := s.now().UTC()
	id, err := identity.New("sub")
	if err != nil {
		return Request{}, err
	}
	status := "pending"
	var reviewedAt *time.Time
	if len(label) >= automaticApprovalMinLength {
		status = "approved"
		reviewedAt = &now
	}
	item := Request{
		ID: id, UserID: userID, Username: username, Label: label,
		FullDomain: label + "." + rootDomain, Status: status, AppliedAt: now,
		ReviewedAt: reviewedAt,
	}
	audit, err := newAudit(userID, username, "subdomain.apply", id, map[string]any{
		"label": label, "automaticallyApproved": status == "approved",
	}, requestID, now)
	if err != nil {
		return Request{}, err
	}
	if err := s.store.Create(ctx, CreateParams{Request: item, Audit: audit}); err != nil {
		return Request{}, err
	}
	return item, nil
}

func (s *Service) Cancel(ctx context.Context, userID, username, requestID string) error {
	if userID == "" || username == "" {
		return ErrInvalidInput
	}
	now := s.now().UTC()
	audit, err := newAudit(userID, username, "subdomain.cancel", userID, nil, requestID, now)
	if err != nil {
		return err
	}
	return s.store.CancelPending(ctx, userID, now, audit)
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

// SetCustomDomain binds an optional CNAME host to the user's approved subdomain.
// Pass empty customDomain to clear. DNS must point the CNAME at FullDomain.
func (s *Service) SetCustomDomain(ctx context.Context, userID, username, customDomain, requestID string) (Request, error) {
	if userID == "" || username == "" {
		return Request{}, ErrInvalidInput
	}
	var domain *string
	customDomain = strings.ToLower(strings.Trim(strings.TrimSpace(customDomain), "."))
	if customDomain != "" {
		if len(customDomain) < 3 || len(customDomain) > 253 || strings.ContainsAny(customDomain, "/: ") || !strings.Contains(customDomain, ".") {
			return Request{}, ErrInvalidInput
		}
		domain = &customDomain
	}
	now := s.now().UTC()
	audit, err := newAudit(userID, username, "subdomain.custom_domain", userID, map[string]any{
		"customDomain": customDomain,
	}, requestID, now)
	if err != nil {
		return Request{}, err
	}
	return s.store.SetCustomDomain(ctx, userID, domain, now, audit)
}

func (s *Service) Review(ctx context.Context, actor Actor, requestID, decision, reason, httpRequestID string) (Request, error) {
	if err := authorize(actor); err != nil {
		return Request{}, err
	}
	reason = strings.TrimSpace(reason)
	if requestID == "" || (decision != "approve" && decision != "reject" && decision != "revoke") || len([]rune(reason)) > 300 {
		return Request{}, ErrInvalidInput
	}
	now := s.now().UTC()
	audit, err := newAudit(actor.ID, actor.Username, "subdomain."+decision, requestID, map[string]any{"reason": reason}, httpRequestID, now)
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
		TargetType: "subdomain", TargetID: targetID, DetailJSON: string(encoded),
		RequestID: requestID, CreatedAt: now,
	}, nil
}
