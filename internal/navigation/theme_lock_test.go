package navigation

import (
	"context"
	"testing"
	"time"

	"github.com/yixian-huang/navax/internal/themes"
)

// 发布锁版本的核心保证：快照记下当时的主题版本，之后主题怎么变都不影响
// 已发布页面。这正是「公开页永远稳定」这条承诺的实现方式。
func TestPublishLocksThemeVersion(t *testing.T) {
	db, service := testNavigationService(t)
	ctx := context.Background()
	actor := insertTestPersonalPage(t, db, "usr_lock_0001", "locker", "pg_lock_0001", "cat_lock_001", "lock-page")

	publication, err := service.Publish(ctx, actor, "pg_lock_0001", 0, "https://nav.ax")
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if publication.SnapshotID == nil {
		t.Fatal("Publish() produced no snapshot")
	}

	published, err := service.PublicBySlug(ctx, "lock-page")
	if err != nil {
		t.Fatalf("PublicBySlug() error = %v", err)
	}
	locked := published.ThemeVersionID
	if locked == "" {
		t.Fatal("published snapshot carries no theme version")
	}

	// 快照的引用也要落在可查询的列上，而不是只藏在 payload_json 里——
	// 藏在 JSON 里数据库管不着，删版本时也就拦不住。
	var column string
	if err := db.QueryRow(`SELECT theme_version_id FROM published_snapshots WHERE id = ?`,
		*publication.SnapshotID).Scan(&column); err != nil {
		t.Fatalf("read theme_version_id column: %v", err)
	}
	if column != locked {
		t.Fatalf("column = %q, payload = %q", column, locked)
	}

	// 被快照引用的版本不得删除。
	if _, err := db.Exec(`DELETE FROM theme_versions WHERE id = ?`, locked); err == nil {
		t.Fatal("deleting a snapshot-referenced theme version must be rejected")
	}

	// 主题换了当前版本，已发布快照仍指向旧版本。
	store := themes.NewStore(db)
	bumped := themes.Package{}
	packages, err := themes.BuiltinPackages()
	if err != nil {
		t.Fatalf("BuiltinPackages() error = %v", err)
	}
	for _, pkg := range packages {
		if pkg.Manifest.ID == published.Settings.Appearance.ThemeID {
			bumped = pkg
		}
	}
	bumped.CSS = append(bumped.CSS, []byte("\n[data-nx=\"clock\"]{opacity:0.99}")...)
	compiled, err := themes.Compile(bumped, bumped.Manifest.ID)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	newVersion, err := store.UpsertVersion(ctx, bumped.Manifest.ID, compiled, "builtin", "builtin", time.Now().UTC())
	if err != nil {
		t.Fatalf("UpsertVersion() error = %v", err)
	}
	if newVersion == locked {
		t.Fatal("test bug: bumped theme produced the same version id")
	}

	reread, err := service.PublicBySlug(ctx, "lock-page")
	if err != nil {
		t.Fatalf("PublicBySlug() error = %v", err)
	}
	if reread.ThemeVersionID != locked {
		t.Fatalf("published snapshot drifted: %q → %q", locked, reread.ThemeVersionID)
	}
}
