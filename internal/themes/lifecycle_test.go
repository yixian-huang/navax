package themes

import (
	"testing"
	"time"
)

// 系统目前没有任何删除用户的代码路径(admin 只能改状态)。这里钉住数据库级
// 不变量:拥有版本行的用户被 DELETE 时,themes 行的 ON DELETE CASCADE 会传导
// 到 theme_versions 的 ON DELETE RESTRICT 而被整体拒绝——未来实现账号删除的
// 人会先撞上这个测试,而不是生产事故。清理顺序见 UninstallPrivate。
func TestDeletingUserWithPrivateThemeVersionsIsBlocked(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	seedUser(t, store, "usr_life_0001")
	installSample(t, store, "usr_life_0001", "lilac", 10)

	if _, err := db.Exec(`DELETE FROM users WHERE id = 'usr_life_0001'`); err == nil {
		t.Fatal("deleting a user whose private theme has versions must be rejected by RESTRICT")
	}
	// 正确路径:先卸载(物理删),用户行才可删。
	var themeID string
	if err := db.QueryRow(`SELECT id FROM themes WHERE owner_id = 'usr_life_0001'`).Scan(&themeID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UninstallPrivate(t.Context(), "usr_life_0001", themeID, time.Now().UTC()); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM users WHERE id = 'usr_life_0001'`); err != nil {
		t.Fatalf("user delete after cleanup: %v", err)
	}
}

// TestUninstallPrivatePhysicalDeleteSucceedsWithRevokedCatalogRequest 钉住
// migrations/0016 的 FK 修复:theme_catalog_requests.version_id 必须带
// ON DELETE CASCADE。deleteThemeTx 先删 theme_versions 再删 themes,若该
// 主题曾经有过目录申请(哪怕早已 revoked/rejected,记录仍留着不会被清理),
// 没有这个 CASCADE 的话 version_id 上默认的 RESTRICT 会先于 theme_id 的
// CASCADE 触发——物理卸载,以及配额压力下走同一条 deleteThemeTx 的墓碑
// 回收(reclaimTombstones),都会直接报 FOREIGN KEY constraint failed。
func TestUninstallPrivatePhysicalDeleteSucceedsWithRevokedCatalogRequest(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	seedUser(t, store, "usr_fk_0001")
	installed := installSample(t, store, "usr_fk_0001", "lilac", 10)

	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`
		INSERT INTO theme_catalog_requests(id, theme_id, owner_id, status, reason, version_id, applied_at, reviewed_at)
		VALUES ('tcr_fk_0001', ?, 'usr_fk_0001', 'revoked', '用户撤回申请', ?, ?, ?)`,
		installed.ThemeID, installed.VersionID, stamp, stamp); err != nil {
		t.Fatal(err)
	}

	removed, err := store.UninstallPrivate(t.Context(), "usr_fk_0001", installed.ThemeID, time.Now().UTC())
	if err != nil || !removed {
		t.Fatalf("UninstallPrivate() = %v, %v; want physical delete to succeed despite a revoked catalog request", removed, err)
	}
	var themeCount, versionCount, requestCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM themes WHERE id = ?`, installed.ThemeID).Scan(&themeCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM theme_versions WHERE theme_id = ?`, installed.ThemeID).Scan(&versionCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM theme_catalog_requests WHERE theme_id = ?`, installed.ThemeID).Scan(&requestCount); err != nil {
		t.Fatal(err)
	}
	if themeCount != 0 || versionCount != 0 || requestCount != 0 {
		t.Fatalf("expected full cascade delete: themes=%d versions=%d catalog_requests=%d", themeCount, versionCount, requestCount)
	}
}
