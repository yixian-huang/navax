package subdomains

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/yixian-huang/navax/internal/database"
	"github.com/yixian-huang/navax/internal/security"
)

func TestSubdomainLifecycle(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	insertUser(t, db, "usr_admin_sub", "owner", "admin@example.com", "admin", now)
	insertUser(t, db, "usr_alice_sub", "alice", "alice@example.com", "user", now)
	insertUser(t, db, "usr_bobby_sub", "bobby", "bobby@example.com", "user", now)

	service := NewService(NewSQLStore(db))
	service.now = func() time.Time { return now }
	if _, err := service.Apply(ctx, "usr_alice_sub", "alice", "alice-nav", "req-disabled"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("disabled policy error = %v", err)
	}
	if _, err := db.ExecContext(ctx, "UPDATE system_settings SET subdomains_enabled = 1, root_domain = 'nav.ax'"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Apply(ctx, "usr_alice_sub", "alice", "Admin", "req-invalid"); !errors.Is(err, ErrInvalidLabel) {
		t.Fatalf("uppercase label error = %v", err)
	}
	if _, err := service.Apply(ctx, "usr_alice_sub", "alice", "admin", "req-reserved"); !errors.Is(err, ErrReservedLabel) {
		t.Fatalf("reserved label error = %v", err)
	}

	first, err := service.Apply(ctx, "usr_alice_sub", "alice", "ali", "req-apply")
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != "pending" || first.FullDomain != "ali.nav.ax" {
		t.Fatalf("unexpected request: %+v", first)
	}
	if _, err := service.Apply(ctx, "usr_alice_sub", "alice", "another-nav", "req-duplicate-user"); !errors.Is(err, ErrConflict) {
		t.Fatalf("active user conflict = %v", err)
	}
	if _, err := service.Apply(ctx, "usr_bobby_sub", "bobby", "ali", "req-duplicate-label"); !errors.Is(err, ErrConflict) {
		t.Fatalf("active label conflict = %v", err)
	}

	admin := Actor{ID: "usr_admin_sub", Username: "owner", Role: "admin", Status: "active"}
	if _, err := service.Requests(ctx, Actor{ID: "usr_alice_sub", Username: "alice", Role: "user", Status: "active"}, "", 1, 20); !errors.Is(err, ErrForbidden) {
		t.Fatalf("non-admin list error = %v", err)
	}
	page, err := service.Requests(ctx, admin, "pending", 1, 20)
	if err != nil || page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("pending page = %+v, %v", page, err)
	}
	approved, err := service.Review(ctx, admin, first.ID, "approve", "", "req-approve")
	if err != nil || approved.Status != "approved" || approved.ReviewedAt == nil {
		t.Fatalf("approve = %+v, %v", approved, err)
	}
	if _, err := service.Review(ctx, admin, first.ID, "approve", "", "req-again"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("repeat approval error = %v", err)
	}
	if err := service.Cancel(ctx, "usr_alice_sub", "alice", "req-cancel-approved"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("cancel approved error = %v", err)
	}
	revoked, err := service.Review(ctx, admin, first.ID, "revoke", "违规内容", "req-revoke")
	if err != nil || revoked.Status != "revoked" || revoked.Reason != "违规内容" {
		t.Fatalf("revoke = %+v, %v", revoked, err)
	}

	service.now = func() time.Time { return now.Add(time.Hour) }
	second, err := service.Apply(ctx, "usr_alice_sub", "alice", "ali", "req-reapply")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Cancel(ctx, "usr_alice_sub", "alice", "req-cancel"); err != nil {
		t.Fatal(err)
	}
	latest, err := service.Mine(ctx, "usr_alice_sub")
	if err != nil || latest == nil || latest.ID != second.ID || latest.Status != "revoked" || latest.Reason != "用户取消申请" {
		t.Fatalf("latest request = %+v, %v", latest, err)
	}

	var audits int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_logs WHERE target_type = 'subdomain'").Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if audits != 5 {
		t.Fatalf("audit count = %d, want 5", audits)
	}
}

func TestStandardSubdomainIsAutomaticallyApproved(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	insertUser(t, db, "usr_auto_sub", "bobby", "bobby@example.com", "user", now)
	if _, err := db.ExecContext(ctx, "UPDATE system_settings SET subdomains_enabled = 1, root_domain = 'nav.ax'"); err != nil {
		t.Fatal(err)
	}

	service := NewService(NewSQLStore(db))
	service.now = func() time.Time { return now }
	item, err := service.Apply(ctx, "usr_auto_sub", "bobby", "tool", "req-auto")
	if err != nil {
		t.Fatal(err)
	}
	if item.Status != "approved" || item.FullDomain != "tool.nav.ax" || item.ReviewedAt == nil || !item.ReviewedAt.Equal(now) {
		t.Fatalf("auto-approved subdomain = %+v", item)
	}

	stored, err := service.Mine(ctx, "usr_auto_sub")
	if err != nil || stored == nil || stored.Status != "approved" || stored.ReviewedAt == nil {
		t.Fatalf("stored auto-approved subdomain = %+v, %v", stored, err)
	}
}

func insertUser(t *testing.T, db *sql.DB, id, username, email, role string, now time.Time) {
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
