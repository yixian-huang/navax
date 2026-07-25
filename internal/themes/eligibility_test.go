package themes

import (
	"errors"
	"testing"
	"time"
)

// seedPrivateTheme 造一个属于 owner 的私有主题并给它一个当前版本。
// 子项目 A 还没有创建私有主题的入口，但归属谓词现在就必须正确——写错的
// 代价是跨租户泄露，不能等到 B 再补。
func seedPrivateTheme(t *testing.T, store *Store, id, owner string) string {
	t.Helper()
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := store.db.Exec(`INSERT INTO users (id, username, email, password_hash, role, status, created_at, updated_at)
		VALUES (?, ?, ?, 'x', 'user', 'active', ?, ?)
		ON CONFLICT(id) DO NOTHING`, owner, owner, owner+"@example.com", stamp, stamp); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := store.db.Exec(`INSERT INTO themes
		(id, name, version, author, description, mode, preview, enabled, is_default,
		 created_at, updated_at, slug, scope, owner_id, source_type)
		VALUES (?, ?, '1.0.0', 'author', '', 'light', '', 1, 0, ?, ?, ?, 'private', ?, 'upload')`,
		id, id, stamp, stamp, id, owner); err != nil {
		t.Fatalf("seed private theme: %v", err)
	}
	versionID, err := store.UpsertVersion(t.Context(), id, compileFor(t, id), "upload", "digest", time.Now().UTC())
	if err != nil {
		t.Fatalf("UpsertVersion() error = %v", err)
	}
	return versionID
}

func TestResolveEligibleVersionEnforcesOwnership(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	if err := SyncBuiltin(t.Context(), store, time.Now().UTC()); err != nil {
		t.Fatalf("SyncBuiltin() error = %v", err)
	}
	var slateVersion string
	if err := db.QueryRow(`SELECT current_version_id FROM themes WHERE id = 'slate'`).Scan(&slateVersion); err != nil {
		t.Fatalf("read slate version: %v", err)
	}
	// sakura 不是默认主题，专门用来把「别名解析」和「未知主题回落默认」这两条
	// 路径区分开——两者都指向 slate 的话，删掉整个别名循环测试也不会变红。
	var sakuraVersion string
	if err := db.QueryRow(`SELECT current_version_id FROM themes WHERE id = 'sakura'`).Scan(&sakuraVersion); err != nil {
		t.Fatalf("read sakura version: %v", err)
	}
	privateVersion := seedPrivateTheme(t, store, "alice-theme", "usr_alice_0001")

	tests := []struct {
		name    string
		themeID string
		actor   string
		want    string
	}{
		{"目录主题人人可用", "slate", "usr_bob_00001", slateVersion},
		{"匿名可用目录主题", "slate", "", slateVersion},
		{"归属者可用自己的私有主题", "alice-theme", "usr_alice_0001", privateVersion},
		{"他人私有主题回落默认", "alice-theme", "usr_bob_00001", slateVersion},
		{"匿名遇到私有主题回落默认", "alice-theme", "", slateVersion},
		{"未知主题回落默认", "does-not-exist", "usr_alice_0001", slateVersion},
		{"culled 别名回落", "kyoto", "", slateVersion},
		// 别名目标不是默认主题：只有真的走了 themeIDAliases 这条分支才会拿到
		// sakura 的版本；一旦别名表被删空，mochi 会当作未知主题回落到 slate，
		// 从而与这里的期望值不一致，测试会失败。
		{"别名指向非默认主题", "mochi", "", sakuraVersion},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := store.ResolveEligibleVersion(t.Context(), tc.themeID, tc.actor)
			if err != nil {
				t.Fatalf("ResolveEligibleVersion() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("version = %q, want %q", got, tc.want)
			}
		})
	}
}

// 卸载私有主题走的是 enabled = 0。私有分支若漏掉这条判定，已卸载的主题
// 会继续被解析出来，与列表行为不一致。
func TestResolveEligibleVersionRespectsDisabledPrivateTheme(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	if err := SyncBuiltin(t.Context(), store, time.Now().UTC()); err != nil {
		t.Fatalf("SyncBuiltin() error = %v", err)
	}
	privateVersion := seedPrivateTheme(t, store, "alice-theme", "usr_alice_0001")

	got, err := store.ResolveEligibleVersion(t.Context(), "alice-theme", "usr_alice_0001")
	if err != nil || got != privateVersion {
		t.Fatalf("before uninstall: version = %q err = %v", got, err)
	}

	if _, err := db.Exec(`UPDATE themes SET enabled = 0 WHERE id = 'alice-theme'`); err != nil {
		t.Fatalf("uninstall private theme: %v", err)
	}
	got, err = store.ResolveEligibleVersion(t.Context(), "alice-theme", "usr_alice_0001")
	if err != nil {
		t.Fatalf("ResolveEligibleVersion() error = %v", err)
	}
	if got == privateVersion {
		t.Fatal("uninstalled private theme must not resolve to its own version")
	}
}

// 撤销走的是 theme_versions.status，而不是 themes.enabled——真实的 kill
// switch 只下架某一个版本，主题本身（含它的 slug、私有归属等）保持原样。
// 直接 UPDATE theme_versions.status 不会触发 themes_current_version_valid
// （该触发器只守 current_version_id 自身的写入），所以这是 EligibilityWhere
// 里 `theme_versions.status = 'active'` 那条腿唯一的覆盖路径。
func TestResolveEligibleVersionFallsBackWhenCurrentVersionRevoked(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	if err := SyncBuiltin(t.Context(), store, time.Now().UTC()); err != nil {
		t.Fatalf("SyncBuiltin() error = %v", err)
	}
	var slateVersion string
	if err := db.QueryRow(`SELECT current_version_id FROM themes WHERE id = 'slate'`).Scan(&slateVersion); err != nil {
		t.Fatalf("read slate version: %v", err)
	}
	var sakuraVersion string
	if err := db.QueryRow(`SELECT current_version_id FROM themes WHERE id = 'sakura'`).Scan(&sakuraVersion); err != nil {
		t.Fatalf("read sakura version: %v", err)
	}

	got, err := store.ResolveEligibleVersion(t.Context(), "sakura", "")
	if err != nil || got != sakuraVersion {
		t.Fatalf("before revocation: version = %q err = %v", got, err)
	}

	if _, err := db.Exec(`UPDATE theme_versions SET status = 'disabled' WHERE id = ?`, sakuraVersion); err != nil {
		t.Fatalf("revoke sakura current version: %v", err)
	}

	got, err = store.ResolveEligibleVersion(t.Context(), "sakura", "")
	if err != nil {
		t.Fatalf("ResolveEligibleVersion() error = %v", err)
	}
	if got != slateVersion {
		t.Fatalf("version = %q, want default (slate) version %q after current version was revoked", got, slateVersion)
	}

	// themes 行本身必须仍是 enabled：这条测试要覆盖的是「版本被撤」，
	// 不是「主题被下架」（那条路径已经有 TestResolveEligibleVersionRespects
	// DisabledPrivateTheme 覆盖）。
	var enabled int
	if err := db.QueryRow(`SELECT enabled FROM themes WHERE id = 'sakura'`).Scan(&enabled); err != nil {
		t.Fatalf("read sakura enabled: %v", err)
	}
	if enabled != 1 {
		t.Fatalf("sakura.enabled = %d, want 1 (only its current version was revoked)", enabled)
	}
}

func TestResolveEligibleVersionFailsLoudlyWhenDefaultBroken(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	if err := SyncBuiltin(t.Context(), store, time.Now().UTC()); err != nil {
		t.Fatalf("SyncBuiltin() error = %v", err)
	}
	if _, err := db.Exec(`UPDATE themes SET enabled = 0`); err != nil {
		t.Fatalf("disable all themes: %v", err)
	}
	if _, err := store.ResolveEligibleVersion(t.Context(), "slate", ""); !errors.Is(err, ErrDefaultThemeUnavailable) {
		t.Fatalf("error = %v, want ErrDefaultThemeUnavailable", err)
	}
}
