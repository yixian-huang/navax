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
