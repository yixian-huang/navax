package auth

import (
	"context"
	"testing"
	"time"

	"github.com/yixian-huang/navax/internal/database"
	"github.com/yixian-huang/navax/internal/identity"
	"github.com/yixian-huang/navax/internal/security"
)

func TestSQLStoreAuthenticationLifecycle(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	store := NewSQLStore(db)
	service := NewService(store, "01234567890123456789012345678901", 24*time.Hour)
	fixedNow := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }

	adminSession, adminToken, err := service.Bootstrap(ctx, "01234567890123456789012345678901", BootstrapInput{
		Username: "owner", Email: "owner@example.com", Password: "strong password",
		InstanceName: "nav.ax", PublicBaseURL: "https://nav.ax",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, adminToken); err != nil {
		t.Fatalf("authenticate bootstrap session: %v", err)
	}

	invitationID, err := identity.New("inv")
	if err != nil {
		t.Fatal(err)
	}
	invitationToken := "valid-invitation-token-for-integration-test"
	_, err = db.ExecContext(ctx, `
		INSERT INTO invitations(id, token_hash, token_preview, creator_id, max_uses, expires_at, created_at)
		VALUES (?, ?, 'valid...', ?, 1, ?, ?)`,
		invitationID, security.HashToken(invitationToken), adminSession.User.ID,
		dbTime(fixedNow.Add(time.Hour)), dbTime(fixedNow),
	)
	if err != nil {
		t.Fatal(err)
	}

	info, err := service.ValidateInvitation(ctx, invitationToken)
	if err != nil || info.InviterName != "owner" {
		t.Fatalf("ValidateInvitation() = %+v, %v", info, err)
	}
	userSession, userToken, err := service.Register(ctx, invitationToken, RegisterInput{
		Username: "alice", Email: "alice@example.com", Password: "another strong password", Device: "integration-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if userSession.User.Role != "user" {
		t.Fatalf("registered role = %q", userSession.User.Role)
	}
	if _, err := service.Authenticate(ctx, userToken); err != nil {
		t.Fatalf("authenticate registered session: %v", err)
	}
	if _, _, err := service.Login(ctx, "alice@example.com", "another strong password", "new-device"); err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if _, err := service.ValidateInvitation(ctx, invitationToken); err != ErrInvitationExhausted {
		t.Fatalf("exhausted invitation error = %v", err)
	}

	var personalPages, uncategorized int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM navigation_pages WHERE kind = 'personal'").Scan(&personalPages); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM categories WHERE is_uncategorized = 1").Scan(&uncategorized); err != nil {
		t.Fatal(err)
	}
	if personalPages != 2 || uncategorized != 3 {
		t.Fatalf("personalPages=%d uncategorized=%d", personalPages, uncategorized)
	}
}
