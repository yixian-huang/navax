package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"math/big"
	"net/mail"
	"strings"
	"time"

	"github.com/yixian-huang/navax/internal/identity"
	"github.com/yixian-huang/navax/internal/security"
)

const (
	emailCodeTTL      = 10 * time.Minute
	emailCodeMaxTries = 5
)

var (
	ErrEmailCodeInvalid = errors.New("email verification code is invalid")
	ErrEmailCodeExpired = errors.New("email verification code is expired")
	ErrMailRequired     = errors.New("email delivery is not configured")
)

type EmailCodePurpose string

const (
	EmailCodeRegister EmailCodePurpose = "register"
	EmailCodeLogin    EmailCodePurpose = "login"
)

// RegisterPayload is stored with a register-purpose code until verified.
type RegisterPayload struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	InvitationToken string `json:"invitationToken,omitempty"`
}

type EmailCodeRecord struct {
	ID        string
	Email     string
	Purpose   EmailCodePurpose
	CodeHash  string
	Payload   string
	ExpiresAt time.Time
	Attempts  int
}

type EmailCodeStore interface {
	CreateEmailCode(ctx context.Context, record EmailCodeRecord, now time.Time) error
	LatestEmailCode(ctx context.Context, email string, purpose EmailCodePurpose, now time.Time) (EmailCodeRecord, error)
	ConsumeEmailCode(ctx context.Context, id string, now time.Time) error
	BumpEmailCodeAttempt(ctx context.Context, id string) error
}

// RequestEmailCode issues a 6-digit code. Returns plain code for emailing (never log).
// For login purpose when the account does not exist, returns ("", nil) so callers
// can still respond with a generic "if the email is registered" message.
func (s *Service) RequestEmailCode(ctx context.Context, email string, purpose EmailCodePurpose, payload RegisterPayload) (plainCode string, err error) {
	email = normalizeEmail(email)
	if _, parseErr := mail.ParseAddress(email); parseErr != nil {
		return "", ErrInvalidCredentials
	}
	switch purpose {
	case EmailCodeRegister:
		if err := validateIdentity(payload.Username, email, payload.Password); err != nil {
			return "", err
		}
		if payload.InvitationToken == "" {
			mode, modeErr := s.store.RegistrationMode(ctx)
			if modeErr != nil {
				return "", modeErr
			}
			if mode != "open" {
				return "", ErrRegistrationClosed
			}
		} else if _, invErr := s.ValidateInvitation(ctx, payload.InvitationToken); invErr != nil {
			return "", invErr
		}
	case EmailCodeLogin:
		user, userErr := s.store.UserByEmail(ctx, email)
		if userErr != nil || user.Status != "active" {
			return "", nil
		}
	default:
		return "", ErrEmailCodeInvalid
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
	payloadJSON := "{}"
	if purpose == EmailCodeRegister {
		raw, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			return "", marshalErr
		}
		payloadJSON = string(raw)
	}
	record := EmailCodeRecord{
		ID: id, Email: email, Purpose: purpose,
		CodeHash: security.HashToken(code), Payload: payloadJSON,
		ExpiresAt: now.Add(emailCodeTTL),
	}
	if err := s.emailStore().CreateEmailCode(ctx, record, now); err != nil {
		return "", err
	}
	return code, nil
}

// VerifyEmailCodeLogin completes passwordless login.
func (s *Service) VerifyEmailCodeLogin(ctx context.Context, email, code, device string) (Session, string, error) {
	email = normalizeEmail(email)
	if err := s.consumeEmailCode(ctx, email, EmailCodeLogin, code); err != nil {
		return Session{}, "", err
	}
	user, err := s.store.UserByEmail(ctx, email)
	if err != nil {
		return Session{}, "", ErrInvalidCredentials
	}
	if user.Status != "active" {
		return Session{}, "", ErrAccountDisabled
	}
	return s.createSession(ctx, user, device)
}

// VerifyEmailCodeRegister completes registration after code verification.
func (s *Service) VerifyEmailCodeRegister(ctx context.Context, email, code, device string) (Session, string, error) {
	email = normalizeEmail(email)
	record, err := s.emailStore().LatestEmailCode(ctx, email, EmailCodeRegister, s.now().UTC())
	if err != nil {
		return Session{}, "", ErrEmailCodeInvalid
	}
	if err := s.verifyCodeRecord(record, code); err != nil {
		return Session{}, "", err
	}
	var payload RegisterPayload
	if err := json.Unmarshal([]byte(record.Payload), &payload); err != nil {
		return Session{}, "", ErrEmailCodeInvalid
	}
	if err := s.emailStore().ConsumeEmailCode(ctx, record.ID, s.now().UTC()); err != nil {
		return Session{}, "", err
	}
	input := RegisterInput{
		Username: payload.Username, Email: email, Password: payload.Password, Device: device,
	}
	if payload.InvitationToken != "" {
		return s.Register(ctx, payload.InvitationToken, input)
	}
	return s.RegisterOpen(ctx, input)
}

func (s *Service) consumeEmailCode(ctx context.Context, email string, purpose EmailCodePurpose, code string) error {
	record, err := s.emailStore().LatestEmailCode(ctx, email, purpose, s.now().UTC())
	if err != nil {
		return ErrEmailCodeInvalid
	}
	if err := s.verifyCodeRecord(record, code); err != nil {
		return err
	}
	return s.emailStore().ConsumeEmailCode(ctx, record.ID, s.now().UTC())
}

func (s *Service) verifyCodeRecord(record EmailCodeRecord, code string) error {
	if record.Attempts >= emailCodeMaxTries {
		return ErrEmailCodeInvalid
	}
	if s.now().UTC().After(record.ExpiresAt) {
		return ErrEmailCodeExpired
	}
	want := security.HashToken(strings.TrimSpace(code))
	if subtle.ConstantTimeCompare([]byte(record.CodeHash), []byte(want)) != 1 {
		_ = s.emailStore().BumpEmailCodeAttempt(context.Background(), record.ID)
		return ErrEmailCodeInvalid
	}
	return nil
}

func (s *Service) emailStore() EmailCodeStore {
	if es, ok := s.store.(EmailCodeStore); ok {
		return es
	}
	panic("auth store does not implement EmailCodeStore")
}

func randomDigits(n int) (string, error) {
	var b strings.Builder
	b.Grow(n)
	for i := 0; i < n; i++ {
		v, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		b.WriteByte(byte('0' + v.Int64()))
	}
	return b.String(), nil
}
