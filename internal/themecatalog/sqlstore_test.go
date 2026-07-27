package themecatalog

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/yixian-huang/navax/internal/database"
	"github.com/yixian-huang/navax/internal/security"
)

func insertUser(t *testing.T, db *sql.DB, id, username, email, role string, now time.Time) {
	t.Helper()
	hash, err := security.HashPassword("integration-password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO users(id, username, email, password_hash, role, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 'active', ?, ?)`, id, username, email, hash, role, dbTime(now), dbTime(now)); err != nil {
		t.Fatal(err)
	}
}

// insertPrivateTheme seeds a minimal private theme + one active current
// version, matching the fixture pattern in internal/admin/sqlstore_test.go.
func insertPrivateTheme(t *testing.T, db *sql.DB, themeID, ownerID, slug, versionID string, now time.Time) {
	t.Helper()
	stamp := dbTime(now)
	if _, err := db.Exec(`INSERT INTO themes (id, name, version, author, description, mode, preview, enabled, is_default,
			created_at, updated_at, slug, scope, owner_id, source_type)
		VALUES (?, ?, '1.0.0', 'owner', '', 'light', '', 1, 0, ?, ?, ?, 'private', ?, 'upload')`,
		themeID, themeID, stamp, stamp, slug, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO theme_versions (id, theme_id, version, source_ref, manifest_json, compiled_css, content_hash, status, created_at)
		VALUES (?, ?, '1.0.0', 'digest', '{}', 'x', ?, 'active', ?)`,
		versionID, themeID, "hash-"+themeID, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE themes SET current_version_id = ? WHERE id = ?`, versionID, themeID); err != nil {
		t.Fatal(err)
	}
}

func TestThemeCatalogRequestLifecycle(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	insertUser(t, db, "usr_admin_tcr", "owner", "admin@example.com", "admin", now)
	insertUser(t, db, "usr_alice_tcr", "alice", "alice@example.com", "user", now)
	insertPrivateTheme(t, db, "thm_alice_tcr", "usr_alice_tcr", "aurora", "vaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", now)

	service := NewService(NewSQLStore(db))
	service.now = func() time.Time { return now }
	admin := Actor{ID: "usr_admin_tcr", Username: "owner", Role: "admin", Status: "active"}
	alice := Actor{ID: "usr_alice_tcr", Username: "alice", Role: "user", Status: "active"}

	// 非本人提交 → ErrNotFound(防枚举)。
	stranger := Actor{ID: "usr_stranger_tcr", Username: "stranger"}
	if _, err := service.Request(ctx, stranger, "thm_alice_tcr", "req-stranger"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("stranger request error = %v", err)
	}

	first, err := service.Request(ctx, alice, "thm_alice_tcr", "req-apply")
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != "pending" || first.Slug != "aurora" || first.ThemeID != "thm_alice_tcr" {
		t.Fatalf("unexpected request: %+v", first)
	}

	// 重复提交(仍 pending)→ ErrConflict。
	if _, err := service.Request(ctx, alice, "thm_alice_tcr", "req-dup"); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate request error = %v", err)
	}

	// 非管理员看队列 → ErrForbidden。
	if _, err := service.Requests(ctx, alice, "", 1, 20); !errors.Is(err, ErrForbidden) {
		t.Fatalf("non-admin list error = %v", err)
	}
	page, err := service.Requests(ctx, admin, "pending", 1, 20)
	if err != nil || page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("pending page = %+v, %v", page, err)
	}

	approved, err := service.Review(ctx, admin, first.ID, "approve", "", "req-approve")
	if err != nil || approved.Status != "approved" {
		t.Fatalf("approve = %+v, %v", approved, err)
	}
	var scope string
	var ownerID sql.NullString
	if err := db.QueryRow(`SELECT scope, owner_id FROM themes WHERE id = ?`, "thm_alice_tcr").Scan(&scope, &ownerID); err != nil {
		t.Fatal(err)
	}
	if scope != "catalog" || ownerID.Valid {
		t.Fatalf("promotion did not flip scope/owner: scope=%q owner=%v", scope, ownerID)
	}

	// 已批准的请求不能再次审批 → ErrInvalidTransition。
	if _, err := service.Review(ctx, admin, first.ID, "approve", "", "req-again"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("repeat approval error = %v", err)
	}

	// slug 冲突:第二个私有主题、同 slug,提交 → ErrSlugConflict。
	insertPrivateTheme(t, db, "thm_bob_tcr", "usr_alice_tcr", "aurora", "vbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", now)
	if _, err := service.Request(ctx, alice, "thm_bob_tcr", "req-slug-conflict"); !errors.Is(err, ErrSlugConflict) {
		t.Fatalf("slug conflict at submission error = %v", err)
	}

	// 拒绝流程:另建一个不同 slug 的主题,拒绝后 owner 仍是 private/自己。
	insertUser(t, db, "usr_carol_tcr", "carol", "carol@example.com", "user", now)
	insertPrivateTheme(t, db, "thm_carol_tcr", "usr_carol_tcr", "borealis", "vcccccccccccccccccccccccccccccccc", now)
	carol := Actor{ID: "usr_carol_tcr", Username: "carol", Role: "user", Status: "active"}
	pending, err := service.Request(ctx, carol, "thm_carol_tcr", "req-carol")
	if err != nil {
		t.Fatal(err)
	}
	rejected, err := service.Review(ctx, admin, pending.ID, "reject", "内容不符合规范", "req-reject")
	if err != nil || rejected.Status != "rejected" || rejected.Reason != "内容不符合规范" {
		t.Fatalf("reject = %+v, %v", rejected, err)
	}
	var carolScope string
	if err := db.QueryRow(`SELECT scope FROM themes WHERE id = ?`, "thm_carol_tcr").Scan(&carolScope); err != nil {
		t.Fatal(err)
	}
	if carolScope != "private" {
		t.Fatalf("rejected theme scope changed: %q", carolScope)
	}
	// 被拒绝后可重新提交。
	resubmitted, err := service.Request(ctx, carol, "thm_carol_tcr", "req-carol-again")
	if err != nil || resubmitted.Status != "pending" {
		t.Fatalf("resubmit after rejection = %+v, %v", resubmitted, err)
	}
	// owner 可撤回自己的 pending 请求。
	if err := service.Cancel(ctx, carol, "thm_carol_tcr", "req-carol-cancel"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Review(ctx, admin, resubmitted.ID, "approve", "", "req-carol-approve-after-cancel"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("approve after cancel error = %v", err)
	}
}
