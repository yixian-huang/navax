package themes

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yixian-huang/navax/internal/database"
)

// newTestDB 建一个跑完全部迁移的内存库。
// MaxOpenConns=1 让 :memory: 的所有语句落在同一个连接上；DSN 里已经带了
// foreign_keys(1)，RESTRICT 才会真正生效——否则本文件里的生命周期断言会假通过。
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.OpenAndMigrate(context.Background(), database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatalf("OpenAndMigrate() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	var foreignKeys int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatalf("query foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}
	return db
}

func compileFor(t *testing.T, packageID string) Compiled {
	t.Helper()
	compiled, err := Compile(samplePackage(t), packageID)
	if err != nil {
		t.Fatalf("Compile(%q) error = %v", packageID, err)
	}
	return compiled
}

func TestUpsertVersionIsIdempotent(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	compiled := compileFor(t, "slate")
	now := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)

	first, err := store.UpsertVersion(t.Context(), "slate", compiled, "builtin", "builtin", now)
	if err != nil {
		t.Fatalf("UpsertVersion() error = %v", err)
	}
	second, err := store.UpsertVersion(t.Context(), "slate", compiled, "builtin", "builtin", now)
	if err != nil {
		t.Fatalf("UpsertVersion() second call error = %v", err)
	}
	if first != second {
		t.Fatalf("version id changed: %q → %q", first, second)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM theme_versions WHERE theme_id = 'slate'`).Scan(&count); err != nil {
		t.Fatalf("count error = %v", err)
	}
	if count != 1 {
		t.Fatalf("theme_versions rows = %d, want 1", count)
	}
	// 重放不得复制资产行。
	var assetCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM theme_assets WHERE theme_version_id = ?`, first).Scan(&assetCount); err != nil {
		t.Fatalf("asset count error = %v", err)
	}
	if assetCount != 1 {
		t.Fatalf("theme_assets rows = %d, want 1", assetCount)
	}

	var currentVersion sql.NullString
	if err := db.QueryRow(`SELECT current_version_id FROM themes WHERE id = 'slate'`).Scan(&currentVersion); err != nil {
		t.Fatalf("current version error = %v", err)
	}
	if currentVersion.String != first {
		t.Fatalf("themes.current_version_id = %q, want %q", currentVersion.String, first)
	}
}

func TestUpsertVersionStoresAssets(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	pkg := samplePackage(t)
	compiled, err := Compile(pkg, "slate")
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	versionID, err := store.UpsertVersion(t.Context(), "slate", compiled, "builtin", "builtin", time.Now().UTC())
	if err != nil {
		t.Fatalf("UpsertVersion() error = %v", err)
	}

	asset, err := store.VersionAsset(t.Context(), versionID, "fonts/sample.woff2")
	if err != nil {
		t.Fatalf("VersionAsset() error = %v", err)
	}
	if asset.MIME != "font/woff2" || !bytes.Equal(asset.Data, pkg.Assets[0].Data) {
		t.Fatalf("asset roundtrip mismatch: mime=%q bytes=%d", asset.MIME, len(asset.Data))
	}
	if asset.SHA256 != pkg.Assets[0].SHA256 {
		t.Fatalf("asset sha256 = %q, want %q", asset.SHA256, pkg.Assets[0].SHA256)
	}
	if _, err := store.VersionAsset(t.Context(), versionID, "fonts/absent.woff2"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}

	css, contentHash, status, err := store.VersionCSS(t.Context(), versionID)
	if err != nil {
		t.Fatalf("VersionCSS() error = %v", err)
	}
	if !bytes.Equal(css, compiled.CSS) || contentHash != compiled.ContentHash || status != "active" {
		t.Fatalf("css roundtrip mismatch: bytes=%d hash=%q status=%q", len(css), contentHash, status)
	}
	if _, _, _, err := store.VersionCSS(t.Context(), "v-does-not-exist"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("VersionCSS() error = %v, want ErrNotFound", err)
	}
}

func TestResolvePackageVersionFallsBack(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	slateVersion, err := store.UpsertVersion(t.Context(), "slate", compileFor(t, "slate"), "builtin", "builtin", time.Now().UTC())
	if err != nil {
		t.Fatalf("UpsertVersion() error = %v", err)
	}
	sakuraVersion, err := store.UpsertVersion(t.Context(), "sakura", compileFor(t, "sakura"), "builtin", "builtin", time.Now().UTC())
	if err != nil {
		t.Fatalf("UpsertVersion(sakura) error = %v", err)
	}

	tests := []struct {
		name    string
		themeID string
		want    string
	}{
		{"已知主题", "slate", slateVersion},
		{"另一个已知主题", "sakura", sakuraVersion},
		{"未知主题回落默认", "does-not-exist", slateVersion},
		{"culled 别名回落", "kyoto", slateVersion},
		{"别名指向非默认主题", "mochi", sakuraVersion},
		{"空 themeId 回落默认", "", slateVersion},
		{"空白 themeId 回落默认", "   ", slateVersion},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := store.ResolvePackageVersion(t.Context(), tc.themeID)
			if err != nil {
				t.Fatalf("ResolvePackageVersion() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("version = %q, want %q", got, tc.want)
			}
		})
	}

	// 当前版本被撤销 → 该主题不再可下发，回落默认主题。
	if _, err := db.Exec(`UPDATE theme_versions SET status = 'disabled' WHERE id = ?`, sakuraVersion); err != nil {
		t.Fatalf("disable version error = %v", err)
	}
	got, err := store.ResolvePackageVersion(t.Context(), "sakura")
	if err != nil {
		t.Fatalf("ResolvePackageVersion() error = %v", err)
	}
	if got != slateVersion {
		t.Fatalf("revoked version resolved to %q, want default %q", got, slateVersion)
	}

	// 回落目标本身不可用时必须明确报错，而不是返回空版本。
	if _, err := db.Exec(`UPDATE theme_versions SET status = 'disabled' WHERE id = ?`, slateVersion); err != nil {
		t.Fatalf("disable default version error = %v", err)
	}
	if _, err := store.ResolvePackageVersion(t.Context(), "slate"); !errors.Is(err, ErrDefaultThemeUnavailable) {
		t.Fatalf("error = %v, want ErrDefaultThemeUnavailable", err)
	}
}

func TestUpsertVersionRejectsUnknownTheme(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	if _, err := store.UpsertVersion(t.Context(), "no-such-theme", compileFor(t, "no-such-theme"), "builtin", "builtin", time.Now().UTC()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestThemesScopeOwnerTriggers(t *testing.T) {
	db := newTestDB(t)
	insertTestUser(t, db, "usr_theme_owner")

	insertTheme := func(id, scope string, ownerID any) error {
		_, err := db.Exec(`
			INSERT INTO themes(id, name, version, author, mode, created_at, updated_at, slug, scope, owner_id)
			VALUES (?, ?, '1.0.0', 'nav.ax', 'light', '2026-07-23T00:00:00Z', '2026-07-23T00:00:00Z', ?, ?, ?)`,
			id, id, id, scope, ownerID)
		return err
	}

	if err := insertTheme("private-orphan", "private", nil); err == nil {
		t.Fatal("private theme without owner_id was accepted")
	} else if !strings.Contains(err.Error(), "owner_id") {
		t.Fatalf("error = %v, want scope/owner trigger abort", err)
	}
	if err := insertTheme("catalog-owned", "catalog", "usr_theme_owner"); err == nil {
		t.Fatal("catalog theme with owner_id was accepted")
	} else if !strings.Contains(err.Error(), "owner_id") {
		t.Fatalf("error = %v, want scope/owner trigger abort", err)
	}

	if err := insertTheme("private-ok", "private", "usr_theme_owner"); err != nil {
		t.Fatalf("valid private theme rejected: %v", err)
	}
	if err := insertTheme("catalog-ok", "catalog", nil); err != nil {
		t.Fatalf("valid catalog theme rejected: %v", err)
	}

	// 更新同样受约束：不能把私有主题的 owner 抹掉再绕过 slug 唯一索引。
	if _, err := db.Exec(`UPDATE themes SET owner_id = NULL WHERE id = 'private-ok'`); err == nil {
		t.Fatal("clearing owner_id on a private theme was accepted")
	}
}

// INSERT 与 UPDATE 必须受同一个守卫。只守 UPDATE 的话，一条新 themes 行可以
// 直接带着指向不存在版本的 current_version_id 插进来，而「绕过服务层的写入」
// 正是这些触发器存在的理由。
func TestCurrentVersionTriggerCoversInsert(t *testing.T) {
	db := newTestDB(t)
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := db.Exec(`INSERT INTO themes
		(id, name, version, author, description, mode, preview, enabled, is_default,
		 created_at, updated_at, slug, scope, current_version_id)
		VALUES ('probe','Probe','1.0.0','x','','light','',1,0,?,?,'probe','catalog','vdoesnotexist')`,
		stamp, stamp)
	if err == nil {
		t.Fatal("INSERT with a dangling current_version_id must be rejected")
	}
	if !strings.Contains(err.Error(), "current_version_id") {
		t.Fatalf("error = %v, want the current_version_id guard", err)
	}
}

func TestCurrentVersionTriggers(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	slateVersion, err := store.UpsertVersion(t.Context(), "slate", compileFor(t, "slate"), "builtin", "builtin", time.Now().UTC())
	if err != nil {
		t.Fatalf("UpsertVersion() error = %v", err)
	}
	sakuraVersion, err := store.UpsertVersion(t.Context(), "sakura", compileFor(t, "sakura"), "builtin", "builtin", time.Now().UTC())
	if err != nil {
		t.Fatalf("UpsertVersion(sakura) error = %v", err)
	}

	// 1. 指向他主题的版本。
	if _, err := db.Exec(`UPDATE themes SET current_version_id = ? WHERE id = 'slate'`, sakuraVersion); err == nil {
		t.Fatal("cross-theme current_version_id was accepted")
	} else if !strings.Contains(err.Error(), "current_version_id") {
		t.Fatalf("error = %v, want current version trigger abort", err)
	}

	// 2. 指向已撤销的版本。
	if _, err := db.Exec(`UPDATE theme_versions SET status = 'disabled' WHERE id = ?`, sakuraVersion); err != nil {
		t.Fatalf("disable version error = %v", err)
	}
	if _, err := db.Exec(`UPDATE themes SET current_version_id = ? WHERE id = 'sakura'`, sakuraVersion); err == nil {
		t.Fatal("disabled current_version_id was accepted")
	}

	// 3. 删除仍被引用为当前版本的版本行——此时它还没有任何快照引用，
	//    published_snapshots 的外键覆盖不到这段空窗。
	if _, err := db.Exec(`DELETE FROM theme_versions WHERE id = ?`, slateVersion); err == nil {
		t.Fatal("deleting a current version was accepted")
	} else if !strings.Contains(err.Error(), "current version") {
		t.Fatalf("error = %v, want current version delete guard", err)
	}

	// 指针指向 NULL 始终允许，卸载与清理都依赖它。
	if _, err := db.Exec(`UPDATE themes SET current_version_id = NULL WHERE id = 'slate'`); err != nil {
		t.Fatalf("clearing current_version_id error = %v", err)
	}
	if _, err := db.Exec(`DELETE FROM theme_versions WHERE id = ?`, slateVersion); err != nil {
		t.Fatalf("deleting an unreferenced version error = %v", err)
	}
}

func TestDeleteVersionReferencedBySnapshotIsRestricted(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	versionID, err := store.UpsertVersion(t.Context(), "slate", compileFor(t, "slate"), "builtin", "builtin", time.Now().UTC())
	if err != nil {
		t.Fatalf("UpsertVersion() error = %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO published_snapshots(id, page_id, draft_revision, slug, visibility, payload_json, etag, published_at, theme_version_id)
		VALUES ('snap_theme_test', 'page_system_root', 1, 'home', 'public', '{}', 'etag-theme-test', '2026-07-23T00:00:00Z', ?)`,
		versionID); err != nil {
		t.Fatalf("insert snapshot error = %v", err)
	}

	// 先摘掉当前版本指针，确保失败只可能来自快照外键，而不是当前版本触发器。
	if _, err := db.Exec(`UPDATE themes SET current_version_id = NULL WHERE id = 'slate'`); err != nil {
		t.Fatalf("clearing current_version_id error = %v", err)
	}
	if _, err := db.Exec(`DELETE FROM theme_versions WHERE id = ?`, versionID); err == nil {
		t.Fatal("deleting a snapshot-referenced version was accepted")
	}

	// 解除引用之后才允许物理清理。
	if _, err := db.Exec(`DELETE FROM published_snapshots WHERE id = 'snap_theme_test'`); err != nil {
		t.Fatalf("delete snapshot error = %v", err)
	}
	if _, err := db.Exec(`DELETE FROM theme_versions WHERE id = ?`, versionID); err != nil {
		t.Fatalf("delete version after clearing references error = %v", err)
	}
}

func TestDeleteThemeWithVersionsIsRestricted(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	versionID, err := store.UpsertVersion(t.Context(), "slate", compileFor(t, "slate"), "builtin", "builtin", time.Now().UTC())
	if err != nil {
		t.Fatalf("UpsertVersion() error = %v", err)
	}
	if _, err := db.Exec(`DELETE FROM themes WHERE id = 'slate'`); err == nil {
		t.Fatal("deleting a theme with versions was accepted")
	}

	// 资产随版本级联，版本不随主题级联——这正是 RESTRICT 想要的形状。
	if _, err := db.Exec(`UPDATE themes SET current_version_id = NULL WHERE id = 'slate'`); err != nil {
		t.Fatalf("clearing current_version_id error = %v", err)
	}
	if _, err := db.Exec(`DELETE FROM theme_versions WHERE id = ?`, versionID); err != nil {
		t.Fatalf("delete version error = %v", err)
	}
	var assetCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM theme_assets WHERE theme_version_id = ?`, versionID).Scan(&assetCount); err != nil {
		t.Fatalf("asset count error = %v", err)
	}
	if assetCount != 0 {
		t.Fatalf("theme_assets rows after version delete = %d, want 0", assetCount)
	}
	if _, err := db.Exec(`DELETE FROM themes WHERE id = 'slate'`); err != nil {
		t.Fatalf("delete theme after clearing versions error = %v", err)
	}
}

func insertTestUser(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	if _, err := db.Exec(`
		INSERT INTO users(id, username, email, password_hash, role, status, created_at, updated_at)
		VALUES (?, ?, ?, 'hash', 'user', 'active', '2026-07-23T00:00:00Z', '2026-07-23T00:00:00Z')`,
		id, id, id+"@example.com"); err != nil {
		t.Fatalf("insert user error = %v", err)
	}
}
