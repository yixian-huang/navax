package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yixian-huang/navax/internal/database"
)

func TestAccountLifecycle(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	service := NewService(NewSQLStore(db), "01234567890123456789012345678901", 24*time.Hour)
	fixedNow := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }
	first, _, err := service.Bootstrap(ctx, "01234567890123456789012345678901", BootstrapInput{
		Username: "owner", Email: "owner@example.com", Password: "initial-password", InstanceName: "nav.ax", PublicBaseURL: "https://nav.ax",
	})
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := service.Login(ctx, "owner@example.com", "initial-password", "Firefox on macOS")
	if err != nil {
		t.Fatal(err)
	}

	username, bio, avatar := "new_owner", "个人导航", "https://cdn.nav.ax/avatar.png"
	profile, err := service.UpdateProfile(ctx, first.User.ID, ProfilePatch{Username: &username, Bio: &bio, AvatarURL: &avatar})
	if err != nil {
		t.Fatal(err)
	}
	if profile.Username != username || profile.Bio != bio || profile.AvatarURL != avatar {
		t.Fatalf("unexpected profile: %+v", profile)
	}

	sessions, err := service.Sessions(ctx, first.User.ID, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 2 {
		t.Fatalf("sessions = %d, want 2", len(sessions))
	}
	if err := service.ChangePassword(ctx, first.User.ID, first.ID, "initial-password", "replacement-password", true); err != nil {
		t.Fatal(err)
	}
	sessions, err = service.Sessions(ctx, first.User.ID, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || sessions[0].ID != first.ID || !sessions[0].Current {
		t.Fatalf("sessions after password change: %+v", sessions)
	}
	if _, _, err := service.Login(ctx, "owner@example.com", "initial-password", "test"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("old password error = %v", err)
	}
	if _, _, err := service.Login(ctx, "owner@example.com", "replacement-password", "test"); err != nil {
		t.Fatalf("new password login: %v", err)
	}
	if err := service.RevokeSession(ctx, first.User.ID, second.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("already revoked session error = %v", err)
	}
	if err := service.ChangePassword(ctx, first.User.ID, first.ID, "wrong-password", "another-password", true); !errors.Is(err, ErrCurrentPassword) {
		t.Fatalf("wrong current password error = %v", err)
	}
}

func TestProfileValidation(t *testing.T) {
	service := NewService(newFakeStore(), "setup", time.Hour)
	invalid := "bad name"
	if _, err := service.UpdateProfile(context.Background(), "usr_test", ProfilePatch{Username: &invalid}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid username error = %v", err)
	}
	if _, err := service.UpdateProfile(context.Background(), "usr_test", ProfilePatch{}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("empty patch error = %v", err)
	}
}
