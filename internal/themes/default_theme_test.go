package themes

import (
	"testing"
	"time"
)

// 复现生产事故（2026-07-23）：迁移 0013 停用了 culled 主题，却没有把
// is_default 从其中一个（mono）挪走。启动时的不变量断言正确地发现了这个
// 坏状态，但当时的反应是拒绝启动——于是任何处于该状态的既有实例升级即挂。
//
// 正确行为是自愈：把可用的基线主题提升为默认并告警，服务照常启动。
func TestSyncBuiltinHealsUnusableDefaultTheme(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)

	// 生产状态：mono 是默认，但已被 0013 停用，且不在内置包里（无编译版本）。
	if _, err := db.Exec(`UPDATE themes SET is_default = 0 WHERE is_default = 1`); err != nil {
		t.Fatalf("clear default: %v", err)
	}
	if _, err := db.Exec(`UPDATE themes SET is_default = 1, enabled = 0 WHERE id = 'mono'`); err != nil {
		t.Fatalf("seed broken default: %v", err)
	}

	if err := SyncBuiltin(t.Context(), store, time.Now().UTC()); err != nil {
		t.Fatalf("SyncBuiltin() error = %v —— 坏的默认主题必须被自愈，而不是让实例起不来", err)
	}

	var (
		defaultID string
		enabled   bool
		versionID string
	)
	if err := db.QueryRow(`SELECT id, enabled, COALESCE(current_version_id, '')
		FROM themes WHERE is_default = 1`).Scan(&defaultID, &enabled, &versionID); err != nil {
		t.Fatalf("read healed default: %v", err)
	}
	if defaultID == "mono" || !enabled || versionID == "" {
		t.Fatalf("default theme not healed: id=%q enabled=%v version=%q", defaultID, enabled, versionID)
	}
	// 基线主题是首选目标，这样自愈结果可预测。
	if defaultID != BaselineThemeID {
		t.Fatalf("healed default = %q, want the baseline %q", defaultID, BaselineThemeID)
	}
	if err := store.AssertDefaultThemeUsable(t.Context()); err != nil {
		t.Fatalf("AssertDefaultThemeUsable() after heal error = %v", err)
	}
}

// 管理员在后台停用了当前默认主题，重启后同样要能自愈——这条路径与
// 历史迁移无关，是常驻风险。
func TestSyncBuiltinHealsAfterAdminDisablesDefault(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	if err := SyncBuiltin(t.Context(), store, time.Now().UTC()); err != nil {
		t.Fatalf("first SyncBuiltin() error = %v", err)
	}
	if _, err := db.Exec(`UPDATE themes SET is_default = 0 WHERE is_default = 1`); err != nil {
		t.Fatalf("clear default: %v", err)
	}
	if _, err := db.Exec(`UPDATE themes SET is_default = 1, enabled = 0 WHERE id = 'noir'`); err != nil {
		t.Fatalf("make disabled theme default: %v", err)
	}

	if err := SyncBuiltin(t.Context(), store, time.Now().UTC()); err != nil {
		t.Fatalf("SyncBuiltin() error = %v", err)
	}
	if err := store.AssertDefaultThemeUsable(t.Context()); err != nil {
		t.Fatalf("AssertDefaultThemeUsable() error = %v", err)
	}
}

// 自愈只保留唯一默认：idx_themes_single_default 是部分唯一索引，
// 旧默认必须在同一事务里先清掉。
func TestEnsureDefaultThemeKeepsSingleDefault(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	if _, err := db.Exec(`UPDATE themes SET is_default = 0 WHERE is_default = 1`); err != nil {
		t.Fatalf("clear default: %v", err)
	}
	if _, err := db.Exec(`UPDATE themes SET is_default = 1, enabled = 0 WHERE id = 'mono'`); err != nil {
		t.Fatalf("seed broken default: %v", err)
	}
	if err := SyncBuiltin(t.Context(), store, time.Now().UTC()); err != nil {
		t.Fatalf("SyncBuiltin() error = %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM themes WHERE is_default = 1`).Scan(&count); err != nil {
		t.Fatalf("count defaults: %v", err)
	}
	if count != 1 {
		t.Fatalf("default theme count = %d, want exactly 1", count)
	}
}
