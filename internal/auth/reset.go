package auth

import (
	"context"
	"time"

	"github.com/yixian-huang/navax/internal/identity"
	"github.com/yixian-huang/navax/internal/security"
)

// ResetRequest is the outcome of a self-service password reset request. Sent is
// false when no active account matched; the caller must respond identically in
// both cases so the endpoint does not reveal whether an email is registered.
type ResetRequest struct {
	User      User
	Token     string
	ExpiresAt time.Time
	Sent      bool
}

// RequestPasswordReset issues a single-use reset token for the account with the
// given email when one exists and is active. Callers email ResetRequest.Token to
// ResetRequest.User and always return a generic success to the client.
func (s *Service) RequestPasswordReset(ctx context.Context, email string) (ResetRequest, error) {
	email = normalizeEmail(email)
	if email == "" || len(email) > 254 {
		return ResetRequest{}, nil
	}
	user, err := s.store.UserByEmail(ctx, email)
	if err != nil || user.Status != "active" {
		// Unknown or disabled account: no token, no error surfaced to the caller.
		return ResetRequest{}, nil
	}
	token, expiresAt, err := s.issueResetToken(ctx, user.ID)
	if err != nil {
		return ResetRequest{}, err
	}
	return ResetRequest{User: user, Token: token, ExpiresAt: expiresAt, Sent: true}, nil
}

// IssuePasswordReset creates a reset token for a specific user. It is used by
// admin-initiated recovery, which works even when no SMTP provider is
// configured because the reset link is returned to the administrator.
func (s *Service) IssuePasswordReset(ctx context.Context, userID string) (ResetRequest, error) {
	accounts, err := s.accounts()
	if err != nil {
		return ResetRequest{}, err
	}
	user, err := accounts.UserByID(ctx, userID)
	if err != nil {
		return ResetRequest{}, err
	}
	token, expiresAt, err := s.issueResetToken(ctx, user.ID)
	if err != nil {
		return ResetRequest{}, err
	}
	return ResetRequest{User: user, Token: token, ExpiresAt: expiresAt, Sent: true}, nil
}

// ResetPassword sets a new password from a reset token, consuming the token and
// revoking every existing session for the account.
func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) error {
	if token == "" {
		return ErrInvalidResetToken
	}
	if len(newPassword) < 12 || len(newPassword) > 1024 {
		return ErrInvalidInput
	}
	accounts, err := s.accounts()
	if err != nil {
		return err
	}
	passwordHash, err := security.HashPassword(newPassword)
	if err != nil {
		return err
	}
	return accounts.ConsumePasswordResetToken(ctx, security.HashToken(token), passwordHash, s.now().UTC())
}

func (s *Service) issueResetToken(ctx context.Context, userID string) (string, time.Time, error) {
	accounts, err := s.accounts()
	if err != nil {
		return "", time.Time{}, err
	}
	token, hash, err := security.NewToken()
	if err != nil {
		return "", time.Time{}, err
	}
	id, err := identity.New("prt")
	if err != nil {
		return "", time.Time{}, err
	}
	now := s.now().UTC()
	expiresAt := now.Add(s.resetTTL)
	insert := PasswordResetInsert{ID: id, UserID: userID, TokenHash: hash, ExpiresAt: expiresAt, CreatedAt: now}
	if err := accounts.CreatePasswordResetToken(ctx, insert); err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}
