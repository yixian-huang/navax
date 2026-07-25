package themes

import (
	"errors"
	"testing"
	"time"
)

func TestSyncBuiltinIsIdempotent(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	stamp := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)

	if err := SyncBuiltin(t.Context(), store, stamp); err != nil {
		t.Fatalf("SyncBuiltin() error = %v", err)
	}
	var first int
	if err := db.QueryRow(`SELECT COUNT(*) FROM theme_versions`).Scan(&first); err != nil {
		t.Fatalf("count versions: %v", err)
	}
	if first == 0 {
		t.Fatal("SyncBuiltin() stored no versions")
	}

	// 第二次启动不得产生新版本行——幂等键是内容哈希，同样的包编译出同样的字节。
	if err := SyncBuiltin(t.Context(), store, stamp.Add(time.Hour)); err != nil {
		t.Fatalf("SyncBuiltin() second run error = %v", err)
	}
	var second int
	if err := db.QueryRow(`SELECT COUNT(*) FROM theme_versions`).Scan(&second); err != nil {
		t.Fatalf("count versions: %v", err)
	}
	if second != first {
		t.Fatalf("version rows grew from %d to %d across restarts", first, second)
	}
}

func TestSyncBuiltinServesEveryBuiltinTheme(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	if err := SyncBuiltin(t.Context(), store, time.Now().UTC()); err != nil {
		t.Fatalf("SyncBuiltin() error = %v", err)
	}
	packages, err := BuiltinPackages()
	if err != nil {
		t.Fatalf("BuiltinPackages() error = %v", err)
	}
	for _, pkg := range packages {
		t.Run(pkg.Manifest.ID, func(t *testing.T) {
			versionID, resolveErr := store.ResolveEligibleVersion(t.Context(), pkg.Manifest.ID, "")
			if resolveErr != nil {
				t.Fatalf("ResolveEligibleVersion() error = %v", resolveErr)
			}
			css, _, status, cssErr := store.VersionCSS(t.Context(), versionID)
			if cssErr != nil {
				t.Fatalf("VersionCSS() error = %v", cssErr)
			}
			if status != VersionStatusActive || len(css) == 0 {
				t.Fatalf("version %s status=%q len=%d", versionID, status, len(css))
			}
		})
	}
}

// UpsertVersion 必须把 manifest 的展示元数据回写 themes 行——否则列表 API
// 永远吐迁移种子的旧文案（slate 行 mode='both' vs manifest 'light' 一类漂移）。
func TestSyncBuiltinWritesBackManifestMetadata(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	if err := SyncBuiltin(t.Context(), store, time.Now().UTC()); err != nil {
		t.Fatalf("SyncBuiltin() error = %v", err)
	}
	packages, err := BuiltinPackages()
	if err != nil {
		t.Fatalf("BuiltinPackages() error = %v", err)
	}
	for _, pkg := range packages {
		var name, author, description, mode, version string
		var specVersion int
		if err := db.QueryRow(`SELECT name, author, description, mode, version, spec_version FROM themes WHERE id = ?`,
			pkg.Manifest.ID).Scan(&name, &author, &description, &mode, &version, &specVersion); err != nil {
			t.Fatalf("read %s: %v", pkg.Manifest.ID, err)
		}
		if name != pkg.Manifest.Name || mode != pkg.Manifest.Mode || version != pkg.Manifest.Version || specVersion != pkg.Manifest.SpecVersion {
			t.Fatalf("%s 行元数据与 manifest 漂移: name=%q mode=%q version=%q spec=%d", pkg.Manifest.ID, name, mode, version, specVersion)
		}
		if description != pkg.Manifest.Description {
			t.Fatalf("%s description = %q, want %q", pkg.Manifest.ID, description, pkg.Manifest.Description)
		}
		if author != pkg.Manifest.Author {
			t.Fatalf("%s author = %q, want %q", pkg.Manifest.ID, author, pkg.Manifest.Author)
		}
	}
	// 内置包都没有 preview.png，preview 列保持空串。
	var preview string
	if err := db.QueryRow(`SELECT preview FROM themes WHERE id = 'slate'`).Scan(&preview); err != nil {
		t.Fatal(err)
	}
	if preview != "" {
		t.Fatalf("slate preview = %q, want empty", preview)
	}
}

// 回落目标自身不可用时必须响亮失败，否则发布会静默产出取不到样式的快照。
func TestAssertDefaultThemeUsableDetectsBrokenFallback(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	if err := SyncBuiltin(t.Context(), store, time.Now().UTC()); err != nil {
		t.Fatalf("SyncBuiltin() error = %v", err)
	}
	if err := store.AssertDefaultThemeUsable(t.Context()); err != nil {
		t.Fatalf("AssertDefaultThemeUsable() error = %v", err)
	}

	if _, err := db.Exec(`UPDATE themes SET enabled = 0 WHERE is_default = 1`); err != nil {
		t.Fatalf("disable default theme: %v", err)
	}
	if err := store.AssertDefaultThemeUsable(t.Context()); !errors.Is(err, ErrDefaultThemeUnavailable) {
		t.Fatalf("error = %v, want ErrDefaultThemeUnavailable", err)
	}
}

// 一次真实事故:管理员停用了某个内置主题(如 sakura)的当前版本后重启进程。
// SyncBuiltin 每次启动都无条件重放同一份编译产物,按内容哈希幂等回读命中的
// 是那条刚被停用的行。若 upsertVersionTx 对此报错,SyncBuiltin 就失败、
// run.go 把它当致命错误,于是进程陷入无限重启(boot loop)。这个测试要
// 证明:带着一个被停用的内置版本重启,SyncBuiltin 必须干净返回,且不悄悄
// 复活那个版本、不影响未受影响的默认主题(slate)。
func TestSyncBuiltinSurvivesDisabledBuiltinVersion(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	stamp := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)

	if err := SyncBuiltin(t.Context(), store, stamp); err != nil {
		t.Fatalf("SyncBuiltin() first run error = %v", err)
	}

	var sakuraVersionID string
	if err := db.QueryRow(`SELECT current_version_id FROM themes WHERE id = 'sakura'`).Scan(&sakuraVersionID); err != nil {
		t.Fatalf("read sakura current_version_id: %v", err)
	}
	if sakuraVersionID == "" {
		t.Fatal("sakura has no current version after first sync")
	}

	// 模拟管理员停用:直接改行,绕开 service 层(与 upsertVersionTx 里
	// 校验的场景一致——重同步撞见的是一条已经是 disabled 的既存行)。
	if _, err := db.Exec(`UPDATE theme_versions SET status = 'disabled' WHERE id = ?`, sakuraVersionID); err != nil {
		t.Fatalf("disable sakura version: %v", err)
	}

	// 重启:再次 SyncBuiltin 必须无错返回,不能把停用当成致命错误。
	if err := SyncBuiltin(t.Context(), store, stamp.Add(time.Hour)); err != nil {
		t.Fatalf("SyncBuiltin() second run (with disabled builtin version) error = %v", err)
	}

	// 该版本仍是 disabled——重同步不得复活它,否则停用形同虚设。
	var status string
	if err := db.QueryRow(`SELECT status FROM theme_versions WHERE id = ?`, sakuraVersionID).Scan(&status); err != nil {
		t.Fatalf("read sakura version status: %v", err)
	}
	if status != VersionStatusDisabled {
		t.Fatalf("sakura version status = %q, want %q (must not be revived)", status, VersionStatusDisabled)
	}

	// current_version_id 也不能被悄悄指回这个 disabled 版本。
	var themeVersionID, themeStatus string
	if err := db.QueryRow(`
		SELECT themes.current_version_id, theme_versions.status
		FROM themes JOIN theme_versions ON theme_versions.id = themes.current_version_id
		WHERE themes.id = 'sakura'`).Scan(&themeVersionID, &themeStatus); err != nil {
		t.Fatalf("read sakura theme pointer: %v", err)
	}
	if themeVersionID != sakuraVersionID || themeStatus != VersionStatusDisabled {
		t.Fatalf("sakura current_version_id should still point at the disabled version %q, got %q (status %q)",
			sakuraVersionID, themeVersionID, themeStatus)
	}

	// 默认主题(slate)未受影响,启动断言照常通过。
	if err := store.AssertDefaultThemeUsable(t.Context()); err != nil {
		t.Fatalf("AssertDefaultThemeUsable() error = %v", err)
	}
	var defaultID string
	if err := db.QueryRow(`SELECT id FROM themes WHERE is_default = 1`).Scan(&defaultID); err != nil {
		t.Fatalf("read default theme: %v", err)
	}
	if defaultID != BaselineThemeID {
		t.Fatalf("default theme = %q, want unaffected baseline %q", defaultID, BaselineThemeID)
	}
}

func TestSyncBuiltinLeavesImporterNull(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	if err := SyncBuiltin(t.Context(), store, time.Now().UTC()); err != nil {
		t.Fatalf("SyncBuiltin() error = %v", err)
	}
	var nonNull int
	if err := db.QueryRow(`SELECT COUNT(*) FROM theme_versions WHERE imported_by IS NOT NULL`).Scan(&nonNull); err != nil {
		t.Fatal(err)
	}
	if nonNull != 0 {
		t.Fatalf("builtin versions must have NULL imported_by, found %d non-null", nonNull)
	}
}
