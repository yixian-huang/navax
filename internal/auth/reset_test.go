package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yixian-huang/navax/internal/database"
)

func TestPasswordResetFlow(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	service := NewService(NewSQLStore(db), "01234567890123456789012345678901", 24*time.Hour)
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	owner, _, err := service.Bootstrap(ctx, "01234567890123456789012345678901", BootstrapInput{
		Username: "owner", Email: "owner@example.com", Password: "initial-password", InstanceName: "nav.ax", PublicBaseURL: "https://nav.ax",
	})
	if err != nil {
		t.Fatal(err)
	}
	// An active login session exists that the reset must invalidate.
	session, sessionToken, err := service.Login(ctx, "owner@example.com", "initial-password", "Firefox")
	if err != nil {
		t.Fatal(err)
	}

	request, err := service.RequestPasswordReset(ctx, "OWNER@example.com")
	if err != nil || !request.Sent || request.Token == "" {
		t.Fatalf("RequestPasswordReset() = %+v, %v", request, err)
	}
	if request.User.ID != owner.User.ID {
		t.Fatalf("reset issued for wrong user: %s", request.User.ID)
	}

	// A short password is rejected before the token is consumed.
	if err := service.ResetPassword(ctx, request.Token, "short"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("weak password error = %v", err)
	}
	if err := service.ResetPassword(ctx, request.Token, "brand-new-password"); err != nil {
		t.Fatalf("ResetPassword() = %v", err)
	}

	// Old password fails, new password works, and the pre-reset session is gone.
	if _, _, err := service.Login(ctx, "owner@example.com", "initial-password", "x"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("old password error = %v", err)
	}
	if _, _, err := service.Login(ctx, "owner@example.com", "brand-new-password", "x"); err != nil {
		t.Fatalf("new password login: %v", err)
	}
	if _, err := service.Authenticate(ctx, sessionToken); err == nil {
		t.Fatalf("session %s survived password reset", session.ID)
	}
	// The token is single-use.
	if err := service.ResetPassword(ctx, request.Token, "yet-another-password"); !errors.Is(err, ErrInvalidResetToken) {
		t.Fatalf("reused token error = %v", err)
	}
}

func TestRequestPasswordResetUnknownEmailIsSilent(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	service := NewService(NewSQLStore(db), "01234567890123456789012345678901", time.Hour)
	if _, _, err := service.Bootstrap(ctx, "01234567890123456789012345678901", BootstrapInput{
		Username: "owner", Email: "owner@example.com", Password: "initial-password", InstanceName: "nav.ax", PublicBaseURL: "https://nav.ax",
	}); err != nil {
		t.Fatal(err)
	}
	request, err := service.RequestPasswordReset(ctx, "stranger@example.com")
	if err != nil {
		t.Fatalf("unknown email error = %v", err)
	}
	if request.Sent || request.Token != "" {
		t.Fatalf("unknown email leaked a token: %+v", request)
	}
}

func TestResetPasswordRejectsExpiredToken(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	service := NewService(NewSQLStore(db), "01234567890123456789012345678901", time.Hour)
	clock := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return clock }
	if _, _, err := service.Bootstrap(ctx, "01234567890123456789012345678901", BootstrapInput{
		Username: "owner", Email: "owner@example.com", Password: "initial-password", InstanceName: "nav.ax", PublicBaseURL: "https://nav.ax",
	}); err != nil {
		t.Fatal(err)
	}
	request, err := service.RequestPasswordReset(ctx, "owner@example.com")
	if err != nil || !request.Sent {
		t.Fatalf("RequestPasswordReset() = %+v, %v", request, err)
	}
	clock = clock.Add(2 * time.Hour) // past the 1h reset TTL
	if err := service.ResetPassword(ctx, request.Token, "brand-new-password"); !errors.Is(err, ErrInvalidResetToken) {
		t.Fatalf("expired token error = %v", err)
	}
}
