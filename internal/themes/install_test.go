package themes

import (
	"errors"
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
