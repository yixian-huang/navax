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
			versionID, resolveErr := store.ResolvePackageVersion(t.Context(), pkg.Manifest.ID)
			if resolveErr != nil {
				t.Fatalf("ResolvePackageVersion() error = %v", resolveErr)
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
