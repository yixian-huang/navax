package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yixian-huang/navax/internal/identity"
	"github.com/yixian-huang/navax/internal/security"
)

var (
	ErrOAuthNotConfigured = errors.New("oauth provider is not configured")
	ErrOAuthDenied        = errors.New("oauth authorization denied")
	ErrOAuthState         = errors.New("oauth state is invalid")
)

type OAuthProvider string

const (
	OAuthGoogle OAuthProvider = "google"
	OAuthGitHub OAuthProvider = "github"
)

type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

// OAuthConfigResolver loads enabled OAuth app credentials.
type OAuthConfigResolver func(ctx context.Context, provider OAuthProvider) (OAuthConfig, bool, error)

type OAuthProfile struct {
	Subject   string
	Email     string
	Username  string
	AvatarURL string
}

type OAuthStore interface {
	SaveOAuthState(ctx context.Context, state, provider, invitationToken string, expiresAt, now time.Time) error
	TakeOAuthState(ctx context.Context, state string, now time.Time) (provider, invitationToken string, err error)
	UserByOAuth(ctx context.Context, provider, subject string) (User, error)
	LinkOAuthIdentity(ctx context.Context, id, provider, subject, userID, email string, now time.Time) error
	CreateOAuthUser(ctx context.Context, params RegistrationParams, provider, subject string, now time.Time) error
}

func (s *Service) SetOAuthResolver(resolve OAuthConfigResolver) {
	s.oauthResolve = resolve
}

// BeginOAuth returns the provider authorize URL and opaque state.
func (s *Service) BeginOAuth(ctx context.Context, provider OAuthProvider, invitationToken string) (authorizeURL, state string, err error) {
	cfg, ok, err := s.oauthConfig(ctx, provider)
	if err != nil {
		return "", "", err
	}
	if !ok {
		return "", "", ErrOAuthNotConfigured
	}
	state, err = randomState()
	if err != nil {
		return "", "", err
	}
	now := s.now().UTC()
	if err := s.oauthStore().SaveOAuthState(ctx, state, string(provider), invitationToken, now.Add(15*time.Minute), now); err != nil {
		return "", "", err
	}
	q := url.Values{}
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", cfg.RedirectURI)
	q.Set("state", state)
	q.Set("response_type", "code")
	switch provider {
	case OAuthGoogle:
		q.Set("scope", "openid email profile")
		q.Set("access_type", "online")
		q.Set("prompt", "select_account")
		return "https://accounts.google.com/o/oauth2/v2/auth?" + q.Encode(), state, nil
	case OAuthGitHub:
		q.Set("scope", "read:user user:email")
		return "https://github.com/login/oauth/authorize?" + q.Encode(), state, nil
	default:
		return "", "", ErrOAuthNotConfigured
	}
}

// OAuthCompleteResult is either an established session or a pending email-OTP
// step for first-time OAuth registration.
type OAuthCompleteResult struct {
	Session      Session
	PlainToken   string
	PendingEmail string // non-empty → redirect to /oauth/complete
	PlainCode    string // only set when PendingEmail is set; for mailer only
	NeedsInvite  bool   // invite-mode and no invitation token yet
}

// CompleteOAuth exchanges the code, finds or links a user, or parks a first-time
// registration behind email verification (+ invitation when required).
func (s *Service) CompleteOAuth(ctx context.Context, provider OAuthProvider, code, state, device string) (OAuthCompleteResult, error) {
	if strings.TrimSpace(code) == "" {
		return OAuthCompleteResult{}, ErrOAuthDenied
	}
	gotProvider, invitationToken, err := s.oauthStore().TakeOAuthState(ctx, state, s.now().UTC())
	if err != nil || gotProvider != string(provider) {
		return OAuthCompleteResult{}, ErrOAuthState
	}
	cfg, ok, err := s.oauthConfig(ctx, provider)
	if err != nil || !ok {
		return OAuthCompleteResult{}, ErrOAuthNotConfigured
	}
	profile, err := exchangeOAuth(ctx, provider, cfg, code)
	if err != nil {
		return OAuthCompleteResult{}, err
	}
	if profile.Subject == "" {
		return OAuthCompleteResult{}, ErrOAuthDenied
	}

	// Existing linked identity.
	if user, userErr := s.oauthStore().UserByOAuth(ctx, string(provider), profile.Subject); userErr == nil {
		if user.Status != "active" {
			return OAuthCompleteResult{}, ErrAccountDisabled
		}
		session, token, err := s.createSession(ctx, user, device)
		if err != nil {
			return OAuthCompleteResult{}, err
		}
		return OAuthCompleteResult{Session: session, PlainToken: token}, nil
	}

	// Link to existing email account when possible.
	email := normalizeEmail(profile.Email)
	if email != "" {
		if user, userErr := s.store.UserByEmail(ctx, email); userErr == nil {
			if user.Status != "active" {
				return OAuthCompleteResult{}, ErrAccountDisabled
			}
			linkID, idErr := identity.New("oai")
			if idErr != nil {
				return OAuthCompleteResult{}, idErr
			}
			if err := s.oauthStore().LinkOAuthIdentity(ctx, linkID, string(provider), profile.Subject, user.ID, email, s.now().UTC()); err != nil {
				return OAuthCompleteResult{}, err
			}
			session, token, err := s.createSession(ctx, user, device)
			if err != nil {
				return OAuthCompleteResult{}, err
			}
			return OAuthCompleteResult{Session: session, PlainToken: token}, nil
		}
	}

	// First-time OAuth: require a real email + OTP before creating the account.
	if email == "" {
		return OAuthCompleteResult{}, ErrOAuthDenied
	}
	mode, err := s.store.RegistrationMode(ctx)
	if err != nil {
		return OAuthCompleteResult{}, err
	}
	if mode == "closed" {
		return OAuthCompleteResult{}, ErrRegistrationClosed
	}
	if invitationToken != "" {
		if _, invErr := s.ValidateInvitation(ctx, invitationToken); invErr != nil {
			return OAuthCompleteResult{}, invErr
		}
	}
	// invite mode without token is OK: complete page collects invitation + OTP.
	needsInvite := invitationToken == "" && mode != "open"
	username := sanitizeOAuthUsername(profile.Username, profile.Email, profile.Subject)
	plainCode, err := s.issueOAuthRegisterCode(ctx, OAuthRegisterPayload{
		Provider: string(provider), Subject: profile.Subject, Email: email,
		Username: username, AvatarURL: strings.TrimSpace(profile.AvatarURL),
		InvitationToken: invitationToken,
	})
	if err != nil {
		return OAuthCompleteResult{}, err
	}
	return OAuthCompleteResult{
		PendingEmail: email, PlainCode: plainCode, NeedsInvite: needsInvite,
	}, nil
}

func (s *Service) issueOAuthRegisterCode(ctx context.Context, payload OAuthRegisterPayload) (plainCode string, err error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	code, err := randomDigits(6)
	if err != nil {
		return "", err
	}
	id, err := identity.New("emc")
	if err != nil {
		return "", err
	}
	now := s.now().UTC()
	record := EmailCodeRecord{
		ID: id, Email: payload.Email, Purpose: EmailCodeOAuthRegister,
		CodeHash: security.HashToken(code), Payload: string(raw),
		ExpiresAt: now.Add(emailCodeTTL),
	}
	if err := s.emailStore().CreateEmailCode(ctx, record, now); err != nil {
		return "", err
	}
	return code, nil
}

// ResendOAuthRegisterCode re-issues OTP for a pending OAuth registration.
func (s *Service) ResendOAuthRegisterCode(ctx context.Context, email string) (plainCode string, needsInvite bool, err error) {
	email = normalizeEmail(email)
	record, err := s.emailStore().LatestEmailCode(ctx, email, EmailCodeOAuthRegister, s.now().UTC())
	if err != nil {
		return "", false, ErrEmailCodeInvalid
	}
	var payload OAuthRegisterPayload
	if err := json.Unmarshal([]byte(record.Payload), &payload); err != nil {
		return "", false, ErrEmailCodeInvalid
	}
	code, err := s.issueOAuthRegisterCode(ctx, payload)
	if err != nil {
		return "", false, err
	}
	mode, modeErr := s.store.RegistrationMode(ctx)
	if modeErr != nil {
		return "", false, modeErr
	}
	needsInvite = payload.InvitationToken == "" && mode != "open"
	return code, needsInvite, nil
}

// VerifyOAuthRegister completes first-time OAuth signup after email OTP
// (and invitation token when the instance is invite-only).
func (s *Service) VerifyOAuthRegister(ctx context.Context, email, code, invitationToken, device string) (Session, string, error) {
	email = normalizeEmail(email)
	record, err := s.emailStore().LatestEmailCode(ctx, email, EmailCodeOAuthRegister, s.now().UTC())
	if err != nil {
		return Session{}, "", ErrEmailCodeInvalid
	}
	if err := s.verifyCodeRecord(record, code); err != nil {
		return Session{}, "", err
	}
	var payload OAuthRegisterPayload
	if err := json.Unmarshal([]byte(record.Payload), &payload); err != nil {
		return Session{}, "", ErrEmailCodeInvalid
	}
	if normalizeEmail(payload.Email) != email {
		return Session{}, "", ErrEmailCodeInvalid
	}
	invite := strings.TrimSpace(invitationToken)
	if invite == "" {
		invite = strings.TrimSpace(payload.InvitationToken)
	}
	mode, err := s.store.RegistrationMode(ctx)
	if err != nil {
		return Session{}, "", err
	}
	if mode == "closed" {
		return Session{}, "", ErrRegistrationClosed
	}
	if invite == "" && mode != "open" {
		return Session{}, "", ErrRegistrationClosed
	}
	if invite != "" {
		if _, invErr := s.ValidateInvitation(ctx, invite); invErr != nil {
			return Session{}, "", invErr
		}
	}
	// Another user may have registered the email while OTP was pending.
	if user, userErr := s.store.UserByEmail(ctx, email); userErr == nil {
		if user.Status != "active" {
			return Session{}, "", ErrAccountDisabled
		}
		if err := s.emailStore().ConsumeEmailCode(ctx, record.ID, s.now().UTC()); err != nil {
			return Session{}, "", err
		}
		linkID, idErr := identity.New("oai")
		if idErr != nil {
			return Session{}, "", idErr
		}
		_ = s.oauthStore().LinkOAuthIdentity(ctx, linkID, payload.Provider, payload.Subject, user.ID, email, s.now().UTC())
		return s.createSession(ctx, user, device)
	}

	randomPass, _, err := security.NewToken()
	if err != nil {
		return Session{}, "", err
	}
	username := payload.Username
	if username == "" {
		username = sanitizeOAuthUsername("", email, payload.Subject)
	}
	input := RegisterInput{Username: username, Email: email, Password: randomPass, Device: device}
	params, plainToken, err := s.prepareRegistration(input)
	if err != nil {
		input.Username = username + "-" + payload.Subject[min(4, len(payload.Subject)):]
		if len(input.Username) > 32 {
			input.Username = input.Username[:32]
		}
		params, plainToken, err = s.prepareRegistration(input)
		if err != nil {
			return Session{}, "", err
		}
	}
	params.User.AvatarURL = strings.TrimSpace(payload.AvatarURL)
	if err := s.emailStore().ConsumeEmailCode(ctx, record.ID, s.now().UTC()); err != nil {
		return Session{}, "", err
	}
	if invite != "" {
		params.InvitationHash = security.HashToken(invite)
		if err := s.store.RegisterWithInvitation(ctx, params, s.now().UTC()); err != nil {
			return Session{}, "", err
		}
		linkID, idErr := identity.New("oai")
		if idErr == nil {
			_ = s.oauthStore().LinkOAuthIdentity(ctx, linkID, payload.Provider, payload.Subject, params.User.ID, email, s.now().UTC())
		}
	} else if err := s.oauthStore().CreateOAuthUser(ctx, params, payload.Provider, payload.Subject, s.now().UTC()); err != nil {
		return Session{}, "", err
	}
	return Session{ID: params.Session.ID, User: params.User, ExpiresAt: params.Session.ExpiresAt}, plainToken, nil
}

// ListEnabledOAuth returns which providers have credentials configured.
func (s *Service) ListEnabledOAuth(ctx context.Context) ([]OAuthProvider, error) {
	out := make([]OAuthProvider, 0, 2)
	for _, p := range []OAuthProvider{OAuthGoogle, OAuthGitHub} {
		_, ok, err := s.oauthConfig(ctx, p)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, p)
		}
	}
	return out, nil
}

func (s *Service) oauthConfig(ctx context.Context, provider OAuthProvider) (OAuthConfig, bool, error) {
	if s.oauthResolve == nil {
		return OAuthConfig{}, false, nil
	}
	return s.oauthResolve(ctx, provider)
}

func (s *Service) oauthStore() OAuthStore {
	if os, ok := s.store.(OAuthStore); ok {
		return os
	}
	panic("auth store does not implement OAuthStore")
}

func exchangeOAuth(ctx context.Context, provider OAuthProvider, cfg OAuthConfig, code string) (OAuthProfile, error) {
	switch provider {
	case OAuthGoogle:
		return exchangeGoogle(ctx, cfg, code)
	case OAuthGitHub:
		return exchangeGitHub(ctx, cfg, code)
	default:
		return OAuthProfile{}, ErrOAuthNotConfigured
	}
}

func exchangeGoogle(ctx context.Context, cfg OAuthConfig, code string) (OAuthProfile, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", cfg.ClientID)
	form.Set("client_secret", cfg.ClientSecret)
	form.Set("redirect_uri", cfg.RedirectURI)
	form.Set("grant_type", "authorization_code")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/token", strings.NewReader(form.Encode()))
	if err != nil {
		return OAuthProfile{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return OAuthProfile{}, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return OAuthProfile{}, fmt.Errorf("%w: token exchange failed", ErrOAuthDenied)
	}
	var token struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &token); err != nil || token.AccessToken == "" {
		return OAuthProfile{}, ErrOAuthDenied
	}
	ureq, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://openidconnect.googleapis.com/v1/userinfo", nil)
	if err != nil {
		return OAuthProfile{}, err
	}
	ureq.Header.Set("Authorization", "Bearer "+token.AccessToken)
	ures, err := http.DefaultClient.Do(ureq)
	if err != nil {
		return OAuthProfile{}, err
	}
	defer ures.Body.Close()
	ubody, _ := io.ReadAll(io.LimitReader(ures.Body, 1<<20))
	var info struct {
		Sub     string `json:"sub"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.Unmarshal(ubody, &info); err != nil {
		return OAuthProfile{}, ErrOAuthDenied
	}
	return OAuthProfile{Subject: info.Sub, Email: info.Email, Username: info.Name, AvatarURL: info.Picture}, nil
}

func exchangeGitHub(ctx context.Context, cfg OAuthConfig, code string) (OAuthProfile, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", cfg.ClientID)
	form.Set("client_secret", cfg.ClientSecret)
	form.Set("redirect_uri", cfg.RedirectURI)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://github.com/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return OAuthProfile{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return OAuthProfile{}, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	var token struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &token); err != nil || token.AccessToken == "" {
		return OAuthProfile{}, ErrOAuthDenied
	}
	ureq, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return OAuthProfile{}, err
	}
	ureq.Header.Set("Authorization", "Bearer "+token.AccessToken)
	ureq.Header.Set("Accept", "application/vnd.github+json")
	ures, err := http.DefaultClient.Do(ureq)
	if err != nil {
		return OAuthProfile{}, err
	}
	defer ures.Body.Close()
	ubody, _ := io.ReadAll(io.LimitReader(ures.Body, 1<<20))
	var info struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.Unmarshal(ubody, &info); err != nil || info.ID == 0 {
		return OAuthProfile{}, ErrOAuthDenied
	}
	email := info.Email
	if email == "" {
		email = fetchGitHubPrimaryEmail(ctx, token.AccessToken)
	}
	return OAuthProfile{
		Subject:   fmt.Sprintf("%d", info.ID),
		Email:     email,
		Username:  info.Login,
		AvatarURL: info.AvatarURL,
	}, nil
}

func fetchGitHubPrimaryEmail(ctx context.Context, accessToken string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.Unmarshal(body, &emails); err != nil {
		return ""
	}
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email
		}
	}
	for _, e := range emails {
		if e.Verified {
			return e.Email
		}
	}
	return ""
}

func randomState() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func sanitizeOAuthUsername(name, email, subject string) string {
	base := strings.TrimSpace(name)
	if base == "" {
		if at := strings.Index(email, "@"); at > 0 {
			base = email[:at]
		} else {
			base = "user"
		}
	}
	var b strings.Builder
	for _, r := range strings.ToLower(base) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if len(out) < 3 {
		out = "user" + subject
		if len(out) > 32 {
			out = out[:32]
		}
	}
	if len(out) > 32 {
		out = out[:32]
	}
	return out
}

func emailOrFallback(email string, provider OAuthProvider, subject string) string {
	if email != "" {
		return email
	}
	return fmt.Sprintf("%s_%s@oauth.local", provider, subject)
}
