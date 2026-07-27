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

	// catalog-scope 主题 → ErrNotFound(防枚举:owner_id=NULL 不应该暴露为内部错误)。
	stamp := dbTime(now)
	if _, err := db.Exec(`INSERT INTO themes (id, name, version, author, description, mode, preview, enabled, is_default,
			created_at, updated_at, slug, scope, owner_id, source_type)
		VALUES (?, 'catalog_theme', '1.0.0', 'owner', '', 'light', '', 1, 0, ?, ?, 'catalog-slug', 'catalog', NULL, 'upload')`,
		"thm_catalog_tcr", stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO theme_versions (id, theme_id, version, source_ref, manifest_json, compiled_css, content_hash, status, created_at)
		VALUES (?, ?, '1.0.0', 'digest', '{}', 'x', 'hash-catalog', 'active', ?)`,
		"vccccccccccccccccccccccccccccccccc", "thm_catalog_tcr", stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE themes SET current_version_id = ? WHERE id = ?`, "vccccccccccccccccccccccccccccccccc", "thm_catalog_tcr"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Request(ctx, alice, "thm_catalog_tcr", "req-catalog-not-found"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("catalog-scope request error = %v", err)
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

	// 审核期间主题被 owner 卸载(墓碑,enabled=0)→ 批准必须拒绝,否则会把一个
	// 已不可见的主题晋升进官方目录。重新启用后审批恢复正常。
	insertUser(t, db, "usr_dave_tcr", "dave", "dave@example.com", "user", now)
	insertPrivateTheme(t, db, "thm_dave_tcr", "usr_dave_tcr", "borealis-two", "vdddddddddddddddddddddddddddddddd", now)
	dave := Actor{ID: "usr_dave_tcr", Username: "dave", Role: "user", Status: "active"}
	davePending, err := service.Request(ctx, dave, "thm_dave_tcr", "req-dave")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE themes SET enabled = 0 WHERE id = ?`, "thm_dave_tcr"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Review(ctx, admin, davePending.ID, "approve", "", "req-dave-approve-tombstoned"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("approve of tombstoned theme error = %v, want ErrInvalidTransition", err)
	}
	if _, err := db.Exec(`UPDATE themes SET enabled = 1 WHERE id = ?`, "thm_dave_tcr"); err != nil {
		t.Fatal(err)
	}
	daveApproved, err := service.Review(ctx, admin, davePending.ID, "approve", "", "req-dave-approve")
	if err != nil || daveApproved.Status != "approved" {
		t.Fatalf("approve after re-enable = %+v, %v", daveApproved, err)
	}

	// 审核期间当前版本被管理员 kill-switch 停用(status='disabled')→ 批准
	// 必须拒绝,不能把一个已停用的版本晋升为官方目录的当前版本。
	insertUser(t, db, "usr_erin_tcr", "erin", "erin@example.com", "user", now)
	insertPrivateTheme(t, db, "thm_erin_tcr", "usr_erin_tcr", "borealis-three", "veeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", now)
	erin := Actor{ID: "usr_erin_tcr", Username: "erin", Role: "user", Status: "active"}
	erinPending, err := service.Request(ctx, erin, "thm_erin_tcr", "req-erin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE theme_versions SET status = 'disabled' WHERE id = ?`, "veeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Review(ctx, admin, erinPending.ID, "approve", "", "req-erin-approve-disabled-version"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("approve of disabled version error = %v, want ErrInvalidTransition", err)
	}
	if _, err := db.Exec(`UPDATE theme_versions SET status = 'active' WHERE id = ?`, "veeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"); err != nil {
		t.Fatal(err)
	}
	erinApproved, err := service.Review(ctx, admin, erinPending.ID, "approve", "", "req-erin-approve")
	if err != nil || erinApproved.Status != "approved" {
		t.Fatalf("approve after version reactivation = %+v, %v", erinApproved, err)
	}
}

// TestThemeCatalogReviewVersionMismatch covers the defensive backstop in
// SQLStore.Review's approve branch: if theme_catalog_requests.version_id no
// longer matches themes.current_version_id when an admin approves, the
// approval must be rejected. Until an upstream "lock upgrades while a
// catalog request is pending" guard exists at the InstallPrivate layer, this
// check is the only thing standing between an admin and approving content
// different from what they reviewed, so it needs its own coverage rather
// than relying on the happy-path test never hitting it.
func TestThemeCatalogReviewVersionMismatch(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	insertUser(t, db, "usr_admin_vm", "owner", "admin-vm@example.com", "admin", now)
	insertUser(t, db, "usr_dave_vm", "dave", "dave@example.com", "user", now)
	insertPrivateTheme(t, db, "thm_dave_vm", "usr_dave_vm", "nebula", "v_dave_1", now)

	service := NewService(NewSQLStore(db))
	service.now = func() time.Time { return now }
	admin := Actor{ID: "usr_admin_vm", Username: "owner", Role: "admin", Status: "active"}
	dave := Actor{ID: "usr_dave_vm", Username: "dave", Role: "user", Status: "active"}

	pending, err := service.Request(ctx, dave, "thm_dave_vm", "req-dave-apply")
	if err != nil {
		t.Fatal(err)
	}

	// 模拟审核期间主题被升级(上游"待审时锁定升级"防护尚未生效的场景):
	// 直接写入第二个 active 版本,并把 current_version_id 改指向它,绕过
	// service 层——这正是 Review 里 version_id 复检要拦住的情况。
	stamp := dbTime(now)
	if _, err := db.Exec(`INSERT INTO theme_versions (id, theme_id, version, source_ref, manifest_json, compiled_css, content_hash, status, created_at)
		VALUES (?, ?, '2.0.0', 'digest', '{}', 'x', ?, 'active', ?)`,
		"v_dave_2", "thm_dave_vm", "hash-thm_dave_vm-v2", stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE themes SET current_version_id = ? WHERE id = ?`, "v_dave_2", "thm_dave_vm"); err != nil {
		t.Fatal(err)
	}

	if _, err := service.Review(ctx, admin, pending.ID, "approve", "", "req-dave-approve"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("approve after version drift error = %v", err)
	}
}

// TestThemeCatalogReviewSlugRaceRecheck covers the live slug re-check inside
// SQLStore.Review's approve branch, as distinct from the submission-time
// check in Create. Submission-time uniqueness is only enforced against
// scope='catalog' themes (idx_themes_private_slug is scoped per-owner), so
// two different owners can each have a pending request for a private theme
// with the same slug. Once the first is approved, approving the second must
// be caught by Review's own re-check, not by anything Create already did.
func TestThemeCatalogReviewSlugRaceRecheck(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	insertUser(t, db, "usr_admin_sr", "owner", "admin-sr@example.com", "admin", now)
	insertUser(t, db, "usr_erin_sr", "erin", "erin@example.com", "user", now)
	insertUser(t, db, "usr_frank_sr", "frank", "frank@example.com", "user", now)
	insertPrivateTheme(t, db, "thm_erin_sr", "usr_erin_sr", "solstice", "v_erin_1", now)
	insertPrivateTheme(t, db, "thm_frank_sr", "usr_frank_sr", "solstice", "v_frank_1", now)

	service := NewService(NewSQLStore(db))
	service.now = func() time.Time { return now }
	admin := Actor{ID: "usr_admin_sr", Username: "owner", Role: "admin", Status: "active"}
	erin := Actor{ID: "usr_erin_sr", Username: "erin", Role: "user", Status: "active"}
	frank := Actor{ID: "usr_frank_sr", Username: "frank", Role: "user", Status: "active"}

	// 两个不同 owner 的私有主题用同一个 slug 提交都应成功:提交期唯一性
	// 只针对 scope='catalog',此时两者都还是 private。
	erinRequest, err := service.Request(ctx, erin, "thm_erin_sr", "req-erin-apply")
	if err != nil {
		t.Fatal(err)
	}
	frankRequest, err := service.Request(ctx, frank, "thm_frank_sr", "req-frank-apply")
	if err != nil {
		t.Fatal(err)
	}

	approved, err := service.Review(ctx, admin, erinRequest.ID, "approve", "", "req-erin-approve")
	if err != nil || approved.Status != "approved" {
		t.Fatalf("first approve = %+v, %v", approved, err)
	}

	// 第二个请求审批时,提交期检查早已通过(当时两者都还是 private),
	// 必须靠 Review 内部的实时复检拦下 slug 冲突。
	if _, err := service.Review(ctx, admin, frankRequest.ID, "approve", "", "req-frank-approve"); !errors.Is(err, ErrSlugConflict) {
		t.Fatalf("second approve slug race error = %v", err)
	}
}
