package admin

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/yixian-huang/navax/internal/database"
	"github.com/yixian-huang/navax/internal/security"
)

func TestAdminManagementLifecycle(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	insertTestUser(t, db, "usr_admin_test", "owner", "owner@example.com", "admin", now)
	insertTestUser(t, db, "usr_alice_test", "alice", "alice@example.com", "user", now.Add(time.Minute))
	insertTestSession(t, db, "ses_alice_test", "usr_alice_test", now)

	store := NewSQLStore(db)
	service := NewService(store)
	service.now = func() time.Time { return now }
	actor := Actor{ID: "usr_admin_test", Username: "owner", Role: "admin", Status: "active"}

	if _, err := service.Users(ctx, Actor{ID: "usr_alice_test", Username: "alice", Role: "user", Status: "active"}, UserFilter{}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("non-admin error = %v", err)
	}
	users, err := service.Users(ctx, actor, UserFilter{Search: "alice", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if users.Total != 1 || len(users.Items) != 1 || users.Items[0].ID != "usr_alice_test" {
		t.Fatalf("unexpected users: %+v", users)
	}
	updated, err := service.SetUserStatus(ctx, actor, "usr_alice_test", "disabled", "离职", "req-disable")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != "disabled" {
		t.Fatalf("status = %q", updated.Status)
	}
	var activeSessions int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE user_id = ? AND revoked_at IS NULL", "usr_alice_test").Scan(&activeSessions); err != nil {
		t.Fatal(err)
	}
	if activeSessions != 0 {
		t.Fatalf("active sessions = %d", activeSessions)
	}
	if _, err := service.SetUserStatus(ctx, actor, actor.ID, "disabled", "", ""); !errors.Is(err, ErrSelfDisable) {
		t.Fatalf("self disable error = %v", err)
	}

	created, err := service.CreateInvitation(ctx, actor, InvitationCreate{
		Email: "guest@example.com", MaxUses: 2, ExpiresInDays: 7, PublicBaseURL: "https://nav.ax/", RequestID: "req-invite",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Token == "" || created.InviteURL == "" || created.TokenPreview == created.Token {
		t.Fatalf("unexpected invitation token response: %+v", created)
	}
	var persistedHash string
	if err := db.QueryRowContext(ctx, "SELECT token_hash FROM invitations WHERE id = ?", created.ID).Scan(&persistedHash); err != nil {
		t.Fatal(err)
	}
	if persistedHash == created.Token || persistedHash != security.HashToken(created.Token) {
		t.Fatal("invitation token was not hashed at rest")
	}
	invitations, err := service.Invitations(ctx, actor, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if invitations.Total != 1 || invitations.Items[0].TokenPreview != created.TokenPreview {
		t.Fatalf("unexpected invitations: %+v", invitations)
	}
	revoked, err := service.RevokeInvitation(ctx, actor, created.ID, "req-revoke")
	if err != nil || revoked.RevokedAt == nil {
		t.Fatalf("revoke invitation = %+v, %v", revoked, err)
	}

	makeDefault := true
	theme, err := service.UpdateTheme(ctx, actor, "kyoto", ThemePatch{Default: &makeDefault, RequestID: "req-theme"})
	if err != nil {
		t.Fatal(err)
	}
	if !theme.Default || !theme.Enabled {
		t.Fatalf("unexpected default theme: %+v", theme)
	}
	disable := false
	if _, err := service.UpdateTheme(ctx, actor, "kyoto", ThemePatch{Enabled: &disable}); !errors.Is(err, ErrDefaultTheme) {
		t.Fatalf("disable default theme error = %v", err)
	}

	name, mode, retention := "我的导航", "closed", 30
	rootDomain := "nav.ax"
	rootDomainValue := &rootDomain
	settings, err := service.UpdateSettings(ctx, actor, SystemSettingsPatch{
		InstanceName: &name, RegistrationMode: &mode,
		Analytics: &AnalyticsPatch{RetentionDays: &retention},
		Domain:    &DomainPatch{RootDomain: &rootDomainValue}, RequestID: "req-settings",
	})
	if err != nil {
		t.Fatal(err)
	}
	if settings.InstanceName != name || settings.Analytics.RetentionDays != retention || settings.Domain.RootDomain == nil || *settings.Domain.RootDomain != rootDomain {
		t.Fatalf("unexpected settings: %+v", settings)
	}

	overview, err := service.Overview(ctx, actor, Health{Status: "healthy", Version: "test", GoVersion: "go1.25"})
	if err != nil {
		t.Fatal(err)
	}
	if overview.TotalUsers != 2 || overview.ActiveUsers != 1 || overview.ActiveInvitations != 0 || len(overview.RecentActions) == 0 {
		t.Fatalf("unexpected overview: %+v", overview)
	}
	audit, err := service.Audit(ctx, actor, AuditFilter{Page: 1, PageSize: 100})
	if err != nil {
		t.Fatal(err)
	}
	if audit.Total < 5 || len(audit.Items) != audit.Total {
		t.Fatalf("unexpected audit page: total=%d items=%d", audit.Total, len(audit.Items))
	}
}

func TestAdminInputValidation(t *testing.T) {
	service := NewService(nil)
	actor := Actor{ID: "usr_admin", Username: "admin", Role: "admin", Status: "active"}
	if _, err := service.CreateInvitation(context.Background(), actor, InvitationCreate{MaxUses: 0, ExpiresInDays: 7, PublicBaseURL: "https://nav.ax"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid invite error = %v", err)
	}
	badMode := "public"
	if _, err := service.UpdateSettings(context.Background(), actor, SystemSettingsPatch{RegistrationMode: &badMode}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid settings error = %v", err)
	}
}

func insertTestUser(t *testing.T, db *sql.DB, id, username, email, role string, now time.Time) {
	t.Helper()
	hash, err := security.HashPassword("integration-password")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		INSERT INTO users(id, username, email, password_hash, role, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 'active', ?, ?)`, id, username, email, hash, role, dbTime(now), dbTime(now))
	if err != nil {
		t.Fatal(err)
	}
}

func insertTestSession(t *testing.T, db *sql.DB, id, userID string, now time.Time) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO sessions(id, user_id, token_hash, device, created_at, last_seen_at, expires_at)
		VALUES (?, ?, ?, 'test', ?, ?, ?)`, id, userID, security.HashToken(id), dbTime(now), dbTime(now), dbTime(now.Add(time.Hour)))
	if err != nil {
		t.Fatal(err)
	}
}
