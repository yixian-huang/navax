package auth

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/yixian-huang/navax/internal/security"
)

var (
	ErrNotFound        = errors.New("not found")
	ErrInvalidInput    = errors.New("invalid input")
	ErrCurrentPassword = errors.New("current password is invalid")
	ErrAccountStore    = errors.New("account operations are unavailable")
)

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`)

// ProfilePatch contains the editable public profile fields. Pointer fields
// distinguish an omitted property from an explicitly empty value.
type ProfilePatch struct {
	Username  *string
	Bio       *string
	AvatarURL *string
}

type SessionInfo struct {
	ID                  string
	Current             bool
	Device              string
	ApproximateLocation string
	CreatedAt           time.Time
	LastSeenAt          time.Time
	ExpiresAt           time.Time
}

// AccountStore is deliberately separate from Store so login/bootstrap mocks
// and alternative authentication stores do not have to implement account
// management until those endpoints are enabled.
type AccountStore interface {
	UserByID(context.Context, string) (User, error)
	UpdateProfile(context.Context, string, ProfilePatch, time.Time) (User, error)
	UpdatePassword(context.Context, string, string, string, bool, time.Time) error
	SessionsByUser(context.Context, string, string, time.Time) ([]SessionInfo, error)
	RevokeOwnedSession(context.Context, string, string, time.Time) error
}

func (s *Service) Profile(ctx context.Context, userID string) (User, error) {
	store, err := s.accounts()
	if err != nil {
		return User{}, err
	}
	return store.UserByID(ctx, userID)
}

func (s *Service) UpdateProfile(ctx context.Context, userID string, patch ProfilePatch) (User, error) {
	if err := validateProfilePatch(patch); err != nil {
		return User{}, err
	}
	store, err := s.accounts()
	if err != nil {
		return User{}, err
	}
	return store.UpdateProfile(ctx, userID, patch, s.now().UTC())
}

func (s *Service) ChangePassword(ctx context.Context, userID, currentSessionID, currentPassword, newPassword string, revokeOtherSessions bool) error {
	if len(newPassword) < 12 || len(newPassword) > 1024 || currentPassword == "" {
		return ErrInvalidInput
	}
	store, err := s.accounts()
	if err != nil {
		return err
	}
	user, err := store.UserByID(ctx, userID)
	if err != nil {
		return err
	}
	valid, verifyErr := security.VerifyPassword(user.PasswordHash, currentPassword)
	if verifyErr != nil || !valid {
		return ErrCurrentPassword
	}
	passwordHash, err := security.HashPassword(newPassword)
	if err != nil {
		return err
	}
	return store.UpdatePassword(ctx, userID, currentSessionID, passwordHash, revokeOtherSessions, s.now().UTC())
}

// VerifyCurrentPassword re-authenticates a signed-in user before a sensitive
// operation without exposing the stored password hash outside the auth layer.
func (s *Service) VerifyCurrentPassword(ctx context.Context, userID, password string) error {
	if userID == "" || password == "" {
		return ErrInvalidInput
	}
	store, err := s.accounts()
	if err != nil {
		return err
	}
	user, err := store.UserByID(ctx, userID)
	if err != nil {
		return err
	}
	valid, verifyErr := security.VerifyPassword(user.PasswordHash, password)
	if verifyErr != nil || !valid {
		return ErrCurrentPassword
	}
	return nil
}

func (s *Service) Sessions(ctx context.Context, userID, currentSessionID string) ([]SessionInfo, error) {
	store, err := s.accounts()
	if err != nil {
		return nil, err
	}
	return store.SessionsByUser(ctx, userID, currentSessionID, s.now().UTC())
}

func (s *Service) RevokeSession(ctx context.Context, userID, sessionID string) error {
	if userID == "" || sessionID == "" {
		return ErrInvalidInput
	}
	store, err := s.accounts()
	if err != nil {
		return err
	}
	return store.RevokeOwnedSession(ctx, userID, sessionID, s.now().UTC())
}

func (s *Service) accounts() (AccountStore, error) {
	store, ok := s.store.(AccountStore)
	if !ok {
		return nil, ErrAccountStore
	}
	return store, nil
}

func validateProfilePatch(patch ProfilePatch) error {
	if patch.Username == nil && patch.Bio == nil && patch.AvatarURL == nil {
		return ErrInvalidInput
	}
	if patch.Username != nil {
		value := strings.TrimSpace(*patch.Username)
		if !usernamePattern.MatchString(value) {
			return ErrInvalidInput
		}
		*patch.Username = value
	}
	if patch.Bio != nil {
		value := strings.TrimSpace(*patch.Bio)
		if len([]rune(value)) > 300 {
			return ErrInvalidInput
		}
		*patch.Bio = value
	}
	if patch.AvatarURL != nil {
		value := strings.TrimSpace(*patch.AvatarURL)
		if len(value) > 2048 {
			return ErrInvalidInput
		}
		if value != "" {
			parsed, err := url.ParseRequestURI(value)
			if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
				return ErrInvalidInput
			}
		}
		*patch.AvatarURL = value
	}
	return nil
}
