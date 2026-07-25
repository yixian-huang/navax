package themes

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"
)

func seedUser(t *testing.T, store *Store, id string) {
	t.Helper()
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := store.db.Exec(`INSERT INTO users (id, username, email, password_hash, role, status, created_at, updated_at)
		VALUES (?, ?, ?, 'x', 'user', 'active', ?, ?) ON CONFLICT(id) DO NOTHING`,
		id, id, id+"@example.com", stamp, stamp); err != nil {
		t.Fatalf("seed user: %v", err)
	}
}

func installSample(t *testing.T, store *Store, ownerID, slug string, quota int) InstalledTheme {
	t.Helper()
	installed, err := store.InstallPrivate(t.Context(), ownerID, slug, "upload", "", "digest-"+slug, quota,
		func(themeID string) (Compiled, error) { return Compile(samplePackage(t), themeID) }, time.Now().UTC())
	if err != nil {
		t.Fatalf("InstallPrivate(%s) error = %v", slug, err)
	}
	return installed
}

func TestInstallPrivateCreatesRowAndVersion(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	seedUser(t, store, "usr_inst_0001")

	installed := installSample(t, store, "usr_inst_0001", "lilac", 10)
	if installed.Upgraded || installed.ThemeID == "" || installed.VersionID == "" || installed.Slug != "lilac" {
		t.Fatalf("unexpected result: %+v", installed)
	}
	var scope, owner, currentVersion, sourceType string
	if err := db.QueryRow(`SELECT scope, owner_id, current_version_id, source_type FROM themes WHERE id = ?`,
		installed.ThemeID).Scan(&scope, &owner, &currentVersion, &sourceType); err != nil {
		t.Fatal(err)
	}
	if scope != "private" || owner != "usr_inst_0001" || currentVersion != installed.VersionID || sourceType != "upload" {
		t.Fatalf("row = %s/%s/%s/%s", scope, owner, currentVersion, sourceType)
	}
	// 装完即可解析(owner 视角),匿名不可见。
	if got, err := store.ResolveEligibleVersion(t.Context(), installed.ThemeID, "usr_inst_0001"); err != nil || got != installed.VersionID {
		t.Fatalf("owner resolve = %q, %v", got, err)
	}
	var slateVersion string
	// 无内置主题时默认回落不可用——本测试库未跑 SyncBuiltin,匿名解析应报默认不可用,
	// 这里只断言拿不到私有版本即可。
	if got, _ := store.ResolveEligibleVersion(t.Context(), installed.ThemeID, ""); got == installed.VersionID {
		t.Fatalf("anonymous must not resolve a private theme, got %q (slate=%q)", got, slateVersion)
	}
}

func TestInstallPrivateSameSlugUpgrades(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	seedUser(t, store, "usr_inst_0002")

	first := installSample(t, store, "usr_inst_0002", "lilac", 10)
	// 变更 CSS 产生新 content hash → 升级为新版本,行数不变。
	upgraded, err := store.InstallPrivate(t.Context(), "usr_inst_0002", "lilac", "upload", "", "digest-2", 10,
		func(themeID string) (Compiled, error) {
			pkg := samplePackage(t)
			pkg.CSS = append(pkg.CSS, []byte("\n[data-nx=\"clock\"] { opacity: 0.9; }")...)
			return Compile(pkg, themeID)
		}, time.Now().UTC())
	if err != nil {
		t.Fatalf("upgrade error = %v", err)
	}
	if !upgraded.Upgraded || upgraded.ThemeID != first.ThemeID || upgraded.VersionID == first.VersionID {
		t.Fatalf("upgrade result: first=%+v upgraded=%+v", first, upgraded)
	}
	var rows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM themes WHERE owner_id = 'usr_inst_0002'`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("themes rows = %d, want 1", rows)
	}
}

func TestInstallPrivateEnforcesQuota(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	seedUser(t, store, "usr_inst_0003")

	installSample(t, store, "usr_inst_0003", "one", 2)
	installSample(t, store, "usr_inst_0003", "two", 2)
	_, err := store.InstallPrivate(t.Context(), "usr_inst_0003", "three", "upload", "", "d3", 2,
		func(themeID string) (Compiled, error) { return Compile(samplePackage(t), themeID) }, time.Now().UTC())
	if !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("err = %v, want ErrQuotaExceeded", err)
	}
	// 升级不占额度:配额已满仍可升级既有 slug。
	if _, err := store.InstallPrivate(t.Context(), "usr_inst_0003", "one", "upload", "", "d4", 2,
		func(themeID string) (Compiled, error) {
			pkg := samplePackage(t)
			pkg.CSS = append(pkg.CSS, []byte("\n[data-nx=\"clock\"] { opacity: 0.8; }")...)
			return Compile(pkg, themeID)
		}, time.Now().UTC()); err != nil {
		t.Fatalf("upgrade at quota error = %v", err)
	}
}

// TestInstallPrivateUpgradeRejectsInvalidSourceType 确认 InstallPrivate 的
// 升级分支——直连 upsertVersionTx、不经过公开包装函数 UpsertVersion——
// 对坏 sourceType 得到的是可读域错误,而不是掉到 SQL 层变成一次不可读的
// 约束冲突(themes.source_type 有 CHECK 约束,但直到 upsertVersionTx 末尾的
// UPDATE 才会触发;校验若不前移,失败信息会是数据库层措辞而不是这里的)。
func TestInstallPrivateUpgradeRejectsInvalidSourceType(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	seedUser(t, store, "usr_inst_bad_source")

	installed := installSample(t, store, "usr_inst_bad_source", "lilac", 10)

	_, err := store.InstallPrivate(t.Context(), "usr_inst_bad_source", "lilac", "not-a-real-source-type", "", "digest-lilac-2", 10,
		func(themeID string) (Compiled, error) { return Compile(samplePackage(t), themeID) }, time.Now().UTC())
	if err == nil {
		t.Fatal("upgrade with an invalid source type was accepted")
	}
	if !strings.Contains(err.Error(), "unsupported source type") {
		t.Fatalf("error = %v, want a readable unsupported-source-type message", err)
	}

	// 主题行未被破坏:被拒的升级整体回滚,current_version_id 仍指向升级前的版本。
	var currentVersion string
	if err := db.QueryRow(`SELECT current_version_id FROM themes WHERE id = ?`, installed.ThemeID).Scan(&currentVersion); err != nil {
		t.Fatal(err)
	}
	if currentVersion != installed.VersionID {
		t.Fatalf("current_version_id = %q, want unchanged %q after rejected upgrade", currentVersion, installed.VersionID)
	}
}

func TestInstallPrivateIsolatesSlugAcrossOwners(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	seedUser(t, store, "usr_inst_a")
	seedUser(t, store, "usr_inst_b")
	a := installSample(t, store, "usr_inst_a", "lilac", 10)
	b := installSample(t, store, "usr_inst_b", "lilac", 10)
	if a.ThemeID == b.ThemeID {
		t.Fatal("different owners with same slug must get distinct theme rows")
	}
}

// seedSnapshotReferencing 造一条引用给定主题版本的已发布快照,用来让
// UninstallPrivate 走「仍被引用」的墓碑分支。复用迁移自带的系统页
// page_system_root(navigation_pages 表,非 brief 草稿里假想的 pages 表)
// 作为宿主页,免得每个测试都要把 navigation_pages 的全部 NOT NULL 列铺一遍。
func seedSnapshotReferencing(t *testing.T, db *sql.DB, snapshotID, versionID string) {
	t.Helper()
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`INSERT INTO published_snapshots
		(id, page_id, draft_revision, slug, visibility, payload_json, etag, published_at, theme_version_id)
		VALUES (?, 'page_system_root', 1, ?, 'public', '{}', ?, ?, ?)`,
		snapshotID, snapshotID, "W/\""+snapshotID+"\"", stamp, versionID); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}
}

func TestUninstallPrivateDeletesUnreferencedTheme(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	seedUser(t, store, "usr_uni_0001")
	installed := installSample(t, store, "usr_uni_0001", "lilac", 10)

	removed, err := store.UninstallPrivate(t.Context(), "usr_uni_0001", installed.ThemeID, time.Now().UTC())
	if err != nil || !removed {
		t.Fatalf("UninstallPrivate() = %v, %v; want removed", removed, err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM themes WHERE id = ?`, installed.ThemeID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("theme row must be physically deleted")
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM theme_versions WHERE theme_id = ?`, installed.ThemeID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("versions must be deleted with the theme")
	}
}

func TestUninstallPrivateTombstonesReferencedTheme(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	seedUser(t, store, "usr_uni_0002")
	installed := installSample(t, store, "usr_uni_0002", "lilac", 10)
	seedSnapshotReferencing(t, db, "snp_uni_0002", installed.VersionID)

	removed, err := store.UninstallPrivate(t.Context(), "usr_uni_0002", installed.ThemeID, time.Now().UTC())
	if err != nil || removed {
		t.Fatalf("UninstallPrivate() = %v, %v; want tombstone", removed, err)
	}
	var enabled bool
	if err := db.QueryRow(`SELECT enabled FROM themes WHERE id = ?`, installed.ThemeID).Scan(&enabled); err != nil {
		t.Fatal(err)
	}
	if enabled {
		t.Fatal("tombstoned theme must be disabled")
	}
	// 公开供应不受影响:版本行仍在且 active。
	if _, _, status, err := store.VersionCSS(t.Context(), installed.VersionID); err != nil || status != VersionStatusActive {
		t.Fatalf("css after tombstone: status=%q err=%v", status, err)
	}
}

func TestUninstallPrivateRejectsForeignTheme(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	seedUser(t, store, "usr_uni_a")
	seedUser(t, store, "usr_uni_b")
	installed := installSample(t, store, "usr_uni_a", "lilac", 10)
	if _, err := store.UninstallPrivate(t.Context(), "usr_uni_b", installed.ThemeID, time.Now().UTC()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign uninstall err = %v, want ErrNotFound", err)
	}
}

// 裁定跟进:重装同 slug 必须能唤醒一个被墓碑化(enabled=0,仍被历史快照
// 引用)的主题行——否则用户卸载后想重新导入同一个主题,会永远卡在一个
// 自己看不见、也无法再启用的僵尸行上。
func TestInstallPrivateReinstallReenablesTombstonedTheme(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	// 需要一个可用的默认主题:墓碑主题解析后应当无错误地回落到它
	// (store.go 的注释与 TestResolveEligibleVersionRespectsDisabledPrivateTheme
	// 都是这个语义),而不是把「默认主题本身不可用」的 ErrDefaultThemeUnavailable
	// 错认成「墓碑生效」的证据。
	if err := SyncBuiltin(t.Context(), store, time.Now().UTC()); err != nil {
		t.Fatalf("SyncBuiltin() error = %v", err)
	}
	seedUser(t, store, "usr_uni_0003")
	first := installSample(t, store, "usr_uni_0003", "lilac", 10)
	seedSnapshotReferencing(t, db, "snp_uni_0003", first.VersionID)

	removed, err := store.UninstallPrivate(t.Context(), "usr_uni_0003", first.ThemeID, time.Now().UTC())
	if err != nil || removed {
		t.Fatalf("UninstallPrivate() = %v, %v; want tombstone", removed, err)
	}
	if got, err := store.ResolveEligibleVersion(t.Context(), first.ThemeID, "usr_uni_0003"); err != nil {
		t.Fatalf("ResolveEligibleVersion() before reinstall error = %v, want fallback to default", err)
	} else if got == first.VersionID {
		t.Fatal("tombstoned theme must not resolve to its own version before reinstall")
	}

	reinstalled, err := store.InstallPrivate(t.Context(), "usr_uni_0003", "lilac", "upload", "", "digest-reinstall", 10,
		func(themeID string) (Compiled, error) {
			pkg := samplePackage(t)
			pkg.CSS = append(pkg.CSS, []byte("\n[data-nx=\"clock\"] { opacity: 0.7; }")...)
			return Compile(pkg, themeID)
		}, time.Now().UTC())
	if err != nil {
		t.Fatalf("reinstall error = %v", err)
	}
	if !reinstalled.Upgraded || reinstalled.ThemeID != first.ThemeID || reinstalled.VersionID == first.VersionID {
		t.Fatalf("reinstall result: first=%+v reinstalled=%+v", first, reinstalled)
	}
	var enabled bool
	if err := db.QueryRow(`SELECT enabled FROM themes WHERE id = ?`, first.ThemeID).Scan(&enabled); err != nil {
		t.Fatal(err)
	}
	if !enabled {
		t.Fatal("reinstalling the same slug must re-enable the tombstoned row")
	}
	if got, err := store.ResolveEligibleVersion(t.Context(), first.ThemeID, "usr_uni_0003"); err != nil || got != reinstalled.VersionID {
		t.Fatalf("owner resolve after reinstall = %q, %v; want %q", got, err, reinstalled.VersionID)
	}
}

func TestInstallPrivateReclaimsUnreferencedTombstonesUnderQuota(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	seedUser(t, store, "usr_recl_0001")

	// 配额=2。装两个:one(将转墓碑,无引用)、two(将转墓碑,有快照引用)。
	one := installSample(t, store, "usr_recl_0001", "one", 2)
	two := installSample(t, store, "usr_recl_0001", "two", 2)

	// 给 two 造一条快照引用,使其卸载后成为"有引用墓碑"。
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`INSERT INTO published_snapshots (id, page_id, draft_revision, slug, visibility, payload_json, etag, published_at, theme_version_id)
		VALUES ('snp_recl_0001', 'page_system_root', 1, 'recl', 'public', '{}', 'W/"x"', ?, ?)`,
		stamp, two.VersionID); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}

	// 卸载两个:one 无引用→物理删(配额腾出);two 有引用→墓碑(占配额)。
	if removed, err := store.UninstallPrivate(t.Context(), "usr_recl_0001", one.ThemeID, time.Now().UTC()); err != nil || !removed {
		t.Fatalf("uninstall one: removed=%v err=%v", removed, err)
	}
	if removed, err := store.UninstallPrivate(t.Context(), "usr_recl_0001", two.ThemeID, time.Now().UTC()); err != nil || removed {
		t.Fatalf("uninstall two: want tombstone, removed=%v err=%v", removed, err)
	}
	// 现在 owner 名下:two(墓碑,占配额)。装 three + four 应无问题(配额 2,占 1)。
	installSample(t, store, "usr_recl_0001", "three", 2)
	// 此刻名下 two(墓碑)+ three = 2,已满。装 four 前若不回收会 ErrQuotaExceeded;
	// 但 two 仍有引用,不可回收 → 确实应满 → four 被拒。
	if _, err := store.InstallPrivate(t.Context(), "usr_recl_0001", "four", "upload", "", "d4", 2,
		func(themeID string) (Compiled, error) { return Compile(samplePackage(t), themeID) }, time.Now().UTC()); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("four should be rejected (two still referenced): %v", err)
	}

	// 移除对 two 的快照引用 → two 变为可回收墓碑。
	if _, err := db.Exec(`DELETE FROM published_snapshots WHERE id = 'snp_recl_0001'`); err != nil {
		t.Fatal(err)
	}
	// 再装 four:配额满 → 回收 two(现无引用)→ 腾出 → 安装成功。
	four, err := store.InstallPrivate(t.Context(), "usr_recl_0001", "four", "upload", "", "d4b", 2,
		func(themeID string) (Compiled, error) { return Compile(samplePackage(t), themeID) }, time.Now().UTC())
	if err != nil {
		t.Fatalf("four should install after reclaiming two: %v", err)
	}
	if four.ThemeID == "" {
		t.Fatal("four not installed")
	}
	// two 已被物理删。
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM themes WHERE id = ?`, two.ThemeID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("reclaimable tombstone two should have been physically deleted")
	}
}
