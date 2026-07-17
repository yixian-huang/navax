package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/yixian-huang/navax/internal/identity"
	"github.com/yixian-huang/navax/internal/security"
)

var (
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrAccountDisabled     = errors.New("account disabled")
	ErrTooManyAttempts     = errors.New("too many login attempts")
	ErrAlreadyInitialized  = errors.New("instance already initialized")
	ErrInvalidSetupToken   = errors.New("invalid setup token")
	ErrInvitationInvalid   = errors.New("invitation is invalid")
	ErrInvitationExpired   = errors.New("invitation is expired")
	ErrInvitationExhausted = errors.New("invitation is exhausted")
	ErrRegistrationClosed  = errors.New("registration is closed")
	ErrConflict            = errors.New("identity conflict")
)

// ThrottledError is returned by Login when the per-account backoff has engaged.
// It carries the remaining wait so the HTTP layer can set a Retry-After header.
type ThrottledError struct {
	RetryAfter time.Duration
}

func (e *ThrottledError) Error() string { return "too many login attempts" }

func (e *ThrottledError) Is(target error) bool { return target == ErrTooManyAttempts }

type User struct {
	ID           string
	Username     string
	Email        string
	PasswordHash string
	AvatarURL    string
	Bio          string
	Role         string
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Session struct {
	ID        string
	User      User
	ExpiresAt time.Time
}

type InvitationInfo struct {
	InviterName string
	ExpiresAt   time.Time
}

type BootstrapInput struct {
	Username      string
	Email         string
	Password      string
	InstanceName  string
	PublicBaseURL string
}

type RegisterInput struct {
	Username string
	Email    string
	Password string
	Device   string
}

type SessionInput struct {
	ID        string
	TokenHash string
	UserID    string
	Device    string
	ExpiresAt time.Time
	Now       time.Time
}

type BootstrapParams struct {
	User            User
	PersonalPageID  string
	UncategorizedID string
	Slug            string
	Session         SessionInput
	InstanceName    string
	PublicBaseURL   string
}

type RegistrationParams struct {
	InvitationHash  string
	User            User
	PageID          string
	UncategorizedID string
	Slug            string
	Session         SessionInput
}

type Store interface {
	Initialized(context.Context) (bool, error)
	Bootstrap(context.Context, BootstrapParams) error
	UserByEmail(context.Context, string) (User, error)
	UserBySessionHash(context.Context, string, time.Time) (Session, error)
	CreateSession(context.Context, SessionInput) error
	DeleteSessionByHash(context.Context, string) error
	InvitationByHash(context.Context, string, time.Time) (InvitationInfo, error)
	RegisterWithInvitation(context.Context, RegistrationParams, time.Time) error
	RegistrationMode(context.Context) (string, error)
	RegisterOpen(context.Context, RegistrationParams, time.Time) error
}

type Service struct {
	store      Store
	setupToken string
	sessionTTL time.Duration
	resetTTL   time.Duration
	now        func() time.Time
	throttle   *loginThrottle
}

func NewService(store Store, setupToken string, sessionTTL time.Duration) *Service {
	return &Service{
		store: store, setupToken: setupToken, sessionTTL: sessionTTL,
		resetTTL: time.Hour, now: time.Now, throttle: newLoginThrottle(),
	}
}

func (s *Service) Initialized(ctx context.Context) (bool, error) {
	return s.store.Initialized(ctx)
}

func (s *Service) Bootstrap(ctx context.Context, providedToken string, input BootstrapInput) (Session, string, error) {
	if subtle.ConstantTimeCompare([]byte(providedToken), []byte(s.setupToken)) != 1 {
		return Session{}, "", ErrInvalidSetupToken
	}
	initialized, err := s.store.Initialized(ctx)
	if err != nil {
		return Session{}, "", err
	}
	if initialized {
		return Session{}, "", ErrAlreadyInitialized
	}
	if err := validateIdentity(input.Username, input.Email, input.Password); err != nil {
		return Session{}, "", err
	}
	input.InstanceName = strings.TrimSpace(input.InstanceName)
	input.PublicBaseURL = strings.TrimRight(strings.TrimSpace(input.PublicBaseURL), "/")
	parsedBaseURL, parseErr := url.ParseRequestURI(input.PublicBaseURL)
	if len(input.InstanceName) < 1 || len([]rune(input.InstanceName)) > 60 || len(input.PublicBaseURL) > 2048 || parseErr != nil || parsedBaseURL.Host == "" || parsedBaseURL.Scheme != "http" && parsedBaseURL.Scheme != "https" {
		return Session{}, "", ErrInvalidInput
	}

	passwordHash, err := security.HashPassword(input.Password)
	if err != nil {
		return Session{}, "", err
	}
	now := s.now().UTC()
	userID, sessionID, pageID, categoryID, err := newBootstrapIDs()
	if err != nil {
		return Session{}, "", err
	}
	plainToken, tokenHash, err := security.NewToken()
	if err != nil {
		return Session{}, "", err
	}
	user := User{
		ID: userID, Username: strings.TrimSpace(input.Username), Email: normalizeEmail(input.Email),
		PasswordHash: passwordHash, Role: "admin", Status: "active", CreatedAt: now, UpdatedAt: now,
	}
	sessionInput := SessionInput{ID: sessionID, TokenHash: tokenHash, UserID: userID, Device: "initial-setup", ExpiresAt: now.Add(s.sessionTTL), Now: now}
	err = s.store.Bootstrap(ctx, BootstrapParams{
		User: user, PersonalPageID: pageID, UncategorizedID: categoryID, Slug: defaultSlug(user.Username, user.ID), Session: sessionInput,
		InstanceName: strings.TrimSpace(input.InstanceName), PublicBaseURL: strings.TrimRight(input.PublicBaseURL, "/"),
	})
	if err != nil {
		return Session{}, "", err
	}
	return Session{ID: sessionID, User: user, ExpiresAt: sessionInput.ExpiresAt}, plainToken, nil
}

func (s *Service) Login(ctx context.Context, email, password, device string) (Session, string, error) {
	if len(email) > 254 || len(password) < 1 || len(password) > 1024 {
		return Session{}, "", ErrInvalidCredentials
	}
	key := normalizeEmail(email)
	now := s.now()
	if retryAfter, blocked := s.throttle.retryAfter(key, now); blocked {
		return Session{}, "", &ThrottledError{RetryAfter: retryAfter}
	}
	user, err := s.store.UserByEmail(ctx, key)
	if err != nil {
		s.throttle.fail(key, now)
		return Session{}, "", ErrInvalidCredentials
	}
	valid, verifyErr := security.VerifyPassword(user.PasswordHash, password)
	if verifyErr != nil || !valid {
		s.throttle.fail(key, now)
		return Session{}, "", ErrInvalidCredentials
	}
	if user.Status != "active" {
		return Session{}, "", ErrAccountDisabled
	}
	s.throttle.success(key)
	return s.createSession(ctx, user, device)
}

func (s *Service) Register(ctx context.Context, invitationToken string, input RegisterInput) (Session, string, error) {
	if invitationToken == "" {
		return Session{}, "", ErrInvitationInvalid
	}
	params, plainToken, err := s.prepareRegistration(input)
	if err != nil {
		return Session{}, "", err
	}
	params.InvitationHash = security.HashToken(invitationToken)
	if err := s.store.RegisterWithInvitation(ctx, params, s.now().UTC()); err != nil {
		return Session{}, "", err
	}
	return Session{ID: params.Session.ID, User: params.User, ExpiresAt: params.Session.ExpiresAt}, plainToken, nil
}

// RegisterOpen creates a user without an invitation when registration_mode is open.
func (s *Service) RegisterOpen(ctx context.Context, input RegisterInput) (Session, string, error) {
	mode, err := s.store.RegistrationMode(ctx)
	if err != nil {
		return Session{}, "", err
	}
	if mode != "open" {
		return Session{}, "", ErrRegistrationClosed
	}
	params, plainToken, err := s.prepareRegistration(input)
	if err != nil {
		return Session{}, "", err
	}
	if err := s.store.RegisterOpen(ctx, params, s.now().UTC()); err != nil {
		return Session{}, "", err
	}
	return Session{ID: params.Session.ID, User: params.User, ExpiresAt: params.Session.ExpiresAt}, plainToken, nil
}

func (s *Service) prepareRegistration(input RegisterInput) (RegistrationParams, string, error) {
	if err := validateIdentity(input.Username, input.Email, input.Password); err != nil {
		return RegistrationParams{}, "", err
	}
	passwordHash, err := security.HashPassword(input.Password)
	if err != nil {
		return RegistrationParams{}, "", err
	}
	now := s.now().UTC()
	userID, err := identity.New("usr")
	if err != nil {
		return RegistrationParams{}, "", err
	}
	pageID, err := identity.New("pag")
	if err != nil {
		return RegistrationParams{}, "", err
	}
	categoryID, err := identity.New("cat")
	if err != nil {
		return RegistrationParams{}, "", err
	}
	sessionID, err := identity.New("ses")
	if err != nil {
		return RegistrationParams{}, "", err
	}
	plainToken, tokenHash, err := security.NewToken()
	if err != nil {
		return RegistrationParams{}, "", err
	}
	user := User{
		ID: userID, Username: strings.TrimSpace(input.Username), Email: normalizeEmail(input.Email),
		PasswordHash: passwordHash, Role: "user", Status: "active", CreatedAt: now, UpdatedAt: now,
	}
	sessionInput := SessionInput{ID: sessionID, TokenHash: tokenHash, UserID: userID, Device: input.Device, ExpiresAt: now.Add(s.sessionTTL), Now: now}
	return RegistrationParams{
		User: user, PageID: pageID, UncategorizedID: categoryID,
		Slug: defaultSlug(user.Username, user.ID), Session: sessionInput,
	}, plainToken, nil
}

func (s *Service) ValidateInvitation(ctx context.Context, invitationToken string) (InvitationInfo, error) {
	if invitationToken == "" {
		return InvitationInfo{}, ErrInvitationInvalid
	}
	return s.store.InvitationByHash(ctx, security.HashToken(invitationToken), s.now().UTC())
}

func (s *Service) Authenticate(ctx context.Context, plainToken string) (Session, error) {
	if plainToken == "" {
		return Session{}, ErrInvalidCredentials
	}
	session, err := s.store.UserBySessionHash(ctx, security.HashToken(plainToken), s.now().UTC())
	if err != nil {
		return Session{}, ErrInvalidCredentials
	}
	if session.User.Status != "active" {
		return Session{}, ErrAccountDisabled
	}
	return session, nil
}

func (s *Service) Logout(ctx context.Context, plainToken string) error {
	if plainToken == "" {
		return nil
	}
	return s.store.DeleteSessionByHash(ctx, security.HashToken(plainToken))
}

func (s *Service) createSession(ctx context.Context, user User, device string) (Session, string, error) {
	plainToken, tokenHash, err := security.NewToken()
	if err != nil {
		return Session{}, "", err
	}
	sessionID, err := identity.New("ses")
	if err != nil {
		return Session{}, "", err
	}
	now := s.now().UTC()
	input := SessionInput{ID: sessionID, TokenHash: tokenHash, UserID: user.ID, Device: device, ExpiresAt: now.Add(s.sessionTTL), Now: now}
	if err := s.store.CreateSession(ctx, input); err != nil {
		return Session{}, "", fmt.Errorf("create session: %w", err)
	}
	return Session{ID: sessionID, User: user, ExpiresAt: input.ExpiresAt}, plainToken, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validateIdentity(username, email, password string) error {
	username = strings.TrimSpace(username)
	email = normalizeEmail(email)
	if !usernamePattern.MatchString(username) || len(email) > 254 || len(password) < 12 || len(password) > 1024 {
		return ErrInvalidInput
	}
	address, err := mail.ParseAddress(email)
	if err != nil || !strings.EqualFold(address.Address, email) {
		return ErrInvalidInput
	}
	return nil
}

func defaultSlug(username, userID string) string {
	base := strings.ToLower(strings.TrimSpace(username))
	base = strings.ReplaceAll(base, "_", "-")
	if len(base) >= 3 && len(base) <= 40 && !reservedSlug(base) {
		return base
	}
	return "user-" + userID[len(userID)-8:]
}

func reservedSlug(value string) bool {
	switch value {
	case "api", "admin", "app", "assets", "invite", "login", "nav", "u", "www":
		return true
	default:
		return false
	}
}

func newBootstrapIDs() (userID, sessionID, pageID, categoryID string, err error) {
	if userID, err = identity.New("usr"); err != nil {
		return "", "", "", "", err
	}
	if sessionID, err = identity.New("ses"); err != nil {
		return "", "", "", "", err
	}
	if pageID, err = identity.New("pag"); err != nil {
		return "", "", "", "", err
	}
	if categoryID, err = identity.New("cat"); err != nil {
		return "", "", "", "", err
	}
	return userID, sessionID, pageID, categoryID, nil
}
