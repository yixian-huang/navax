package admin

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yixian-huang/navax/internal/database"
	"github.com/yixian-huang/navax/internal/security"
	"github.com/yixian-huang/navax/internal/themes"
)

func TestAdminManagementLifecycle(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	insertTestUser(t, db, "usr_admin_test", "owner", "owner@example.com", "admin", now)
	insertTestUser(t, db, "usr_alice_test", "alice", "alice@example.com", "user", now.Add(time.Minute))
	insertTestSession(t, db, "ses_alice_test", "usr_alice_test", now)

	store := NewSQLStore(db)
	service := NewService(store)
	service.now = func() time.Time { return now }
	actor := Actor{ID: "usr_admin_test", Username: "owner", Role: "admin", Status: "active"}

	if _, err := service.Users(ctx, Actor{ID: "usr_alice_test", Username: "alice", Role: "user", Status: "active"}, UserFilter{}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("non-admin error = %v", err)
	}
	users, err := service.Users(ctx, actor, UserFilter{Search: "alice", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if users.Total != 1 || len(users.Items) != 1 || users.Items[0].ID != "usr_alice_test" {
		t.Fatalf("unexpected users: %+v", users)
	}
	updated, err := service.SetUserStatus(ctx, actor, "usr_alice_test", "disabled", "离职", "req-disable")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != "disabled" {
		t.Fatalf("status = %q", updated.Status)
	}
	var activeSessions int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE user_id = ? AND revoked_at IS NULL", "usr_alice_test").Scan(&activeSessions); err != nil {
		t.Fatal(err)
	}
	if activeSessions != 0 {
		t.Fatalf("active sessions = %d", activeSessions)
	}
	if _, err := service.SetUserStatus(ctx, actor, actor.ID, "disabled", "", ""); !errors.Is(err, ErrSelfDisable) {
		t.Fatalf("self disable error = %v", err)
	}

	created, err := service.CreateInvitation(ctx, actor, InvitationCreate{
		Email: "guest@example.com", MaxUses: 2, ExpiresInDays: 7, PublicBaseURL: "https://nav.ax/", RequestID: "req-invite",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Token == "" || created.InviteURL == "" || created.TokenPreview == created.Token {
		t.Fatalf("unexpected invitation token response: %+v", created)
	}
	var persistedHash string
	if err := db.QueryRowContext(ctx, "SELECT token_hash FROM invitations WHERE id = ?", created.ID).Scan(&persistedHash); err != nil {
		t.Fatal(err)
	}
	if persistedHash == created.Token || persistedHash != security.HashToken(created.Token) {
		t.Fatal("invitation token was not hashed at rest")
	}
	invitations, err := service.Invitations(ctx, actor, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if invitations.Total != 1 || invitations.Items[0].TokenPreview != created.TokenPreview {
		t.Fatalf("unexpected invitations: %+v", invitations)
	}
	revoked, err := service.RevokeInvitation(ctx, actor, created.ID, "req-revoke")
	if err != nil || revoked.RevokedAt == nil {
		t.Fatalf("revoke invitation = %+v, %v", revoked, err)
	}

	makeDefault := true
	theme, err := service.UpdateTheme(ctx, actor, "kyoto", ThemePatch{Default: &makeDefault, RequestID: "req-theme"})
	if err != nil {
		t.Fatal(err)
	}
	if !theme.Default || !theme.Enabled {
		t.Fatalf("unexpected default theme: %+v", theme)
	}
	disable := false
	if _, err := service.UpdateTheme(ctx, actor, "kyoto", ThemePatch{Enabled: &disable}); !errors.Is(err, ErrDefaultTheme) {
		t.Fatalf("disable default theme error = %v", err)
	}

	name, mode, retention := "我的导航", "closed", 30
	rootDomain := "nav.ax"
	rootDomainValue := &rootDomain
	settings, err := service.UpdateSettings(ctx, actor, SystemSettingsPatch{
		InstanceName: &name, RegistrationMode: &mode,
		Analytics: &AnalyticsPatch{RetentionDays: &retention},
		Domain:    &DomainPatch{RootDomain: &rootDomainValue}, RequestID: "req-settings",
	})
	if err != nil {
		t.Fatal(err)
	}
	if settings.InstanceName != name || settings.Analytics.RetentionDays != retention || settings.Domain.RootDomain == nil || *settings.Domain.RootDomain != rootDomain {
		t.Fatalf("unexpected settings: %+v", settings)
	}

	overview, err := service.Overview(ctx, actor, Health{Status: "healthy", Version: "test", GoVersion: "go1.25"})
	if err != nil {
		t.Fatal(err)
	}
	if overview.TotalUsers != 2 || overview.ActiveUsers != 1 || overview.ActiveInvitations != 0 || len(overview.RecentActions) == 0 {
		t.Fatalf("unexpected overview: %+v", overview)
	}
	audit, err := service.Audit(ctx, actor, AuditFilter{Page: 1, PageSize: 100})
	if err != nil {
		t.Fatal(err)
	}
	if audit.Total < 5 || len(audit.Items) != audit.Total {
		t.Fatalf("unexpected audit page: total=%d items=%d", audit.Total, len(audit.Items))
	}
}

func TestAdminInputValidation(t *testing.T) {
	service := NewService(nil)
	actor := Actor{ID: "usr_admin", Username: "admin", Role: "admin", Status: "active"}
	if _, err := service.CreateInvitation(context.Background(), actor, InvitationCreate{MaxUses: 0, ExpiresInDays: 7, PublicBaseURL: "https://nav.ax"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid invite error = %v", err)
	}
	badMode := "public"
	if _, err := service.UpdateSettings(context.Background(), actor, SystemSettingsPatch{RegistrationMode: &badMode}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid settings error = %v", err)
	}
}

func insertTestUser(t *testing.T, db *sql.DB, id, username, email, role string, now time.Time) {
	t.Helper()
	hash, err := security.HashPassword("integration-password")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		INSERT INTO users(id, username, email, password_hash, role, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 'active', ?, ?)`, id, username, email, hash, role, dbTime(now), dbTime(now))
	if err != nil {
		t.Fatal(err)
	}
}

func insertTestSession(t *testing.T, db *sql.DB, id, userID string, now time.Time) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO sessions(id, user_id, token_hash, device, created_at, last_seen_at, expires_at)
		VALUES (?, ?, ?, 'test', ?, ?, ?)`, id, userID, security.HashToken(id), dbTime(now), dbTime(now), dbTime(now.Add(time.Hour)))
	if err != nil {
		t.Fatal(err)
	}
}

// 管理后台是全量视图:既要读出有编译版本主题的 manifest 字段(色板、vibe
// 分组都依赖它们),也要保留停用且无版本的行——那是它与 eligibility 谓词
// 的职责区别。
func TestThemeListingIncludesSpecV1Fields(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := themes.SyncBuiltin(ctx, themes.NewStore(db), time.Now().UTC()); err != nil {
		t.Fatalf("SyncBuiltin() error = %v", err)
	}

	store := NewSQLStore(db)
	items, err := store.ListThemes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]Theme{}
	for _, item := range items {
		byID[item.ID] = item
	}

	sakura, ok := byID["sakura"]
	if !ok {
		t.Fatal("列表缺少 sakura")
	}
	if sakura.Vibe != "cute" {
		t.Fatalf("sakura vibe = %q, want cute", sakura.Vibe)
	}
	for i, swatch := range sakura.Swatches {
		if !strings.HasPrefix(swatch, "#") {
			t.Fatalf("sakura swatch[%d] = %q,不是真实色板", i, swatch)
		}
	}
	if sakura.CurrentVersionID == "" ||
		sakura.CSSHref != "/api/v1/public/themes/"+sakura.CurrentVersionID+".css" {
		t.Fatalf("sakura 版本字段未接线: %+v", sakura)
	}
	if sakura.Scope != "catalog" || sakura.Tier != 1 || sakura.Subtitle == "" {
		t.Fatalf("sakura manifest 字段未接线: %+v", sakura)
	}

	// migration 0013 停用的 mono 没有编译版本,必须保留在管理列表且 v1 字段为零值。
	mono, ok := byID["mono"]
	if !ok {
		t.Fatal("管理列表必须包含已停用的 mono")
	}
	if mono.Enabled {
		t.Fatal("mono 应处于停用状态")
	}
	if mono.CurrentVersionID != "" || mono.CSSHref != "" || mono.Vibe != "" || mono.Swatches[0] != "" {
		t.Fatalf("无版本主题的 v1 字段应为零值: %+v", mono)
	}

	single, err := store.Theme(ctx, "sakura")
	if err != nil {
		t.Fatal(err)
	}
	if single.Vibe != "cute" || single.CurrentVersionID == "" {
		t.Fatalf("单行查询未接线: %+v", single)
	}
}

// 默认主题必须是目录主题(设计 §7.1):私有主题只对其所有者可见,把它设为
// 实例默认会让所有其他用户在发布时回落到一个自己看不到、也无权访问的主题。
func TestUpdateThemeRejectsPrivateDefault(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	now := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)
	insertTestUser(t, db, "usr_pd_01", "pduser", "pd@example.com", "user", now)
	stamp := dbTime(now)
	if _, err := db.Exec(`INSERT INTO themes (id, name, version, author, description, mode, preview, enabled, is_default, created_at, updated_at, slug, scope, owner_id, source_type)
		VALUES ('thm_pd_01', 'PD', '1.0.0', 'pd', '', 'light', '', 1, 0, ?, ?, 'pd', 'private', 'usr_pd_01', 'upload')`, stamp, stamp); err != nil {
		t.Fatal(err)
	}

	store := NewSQLStore(db)
	enable := true
	audit := AuditRecord{AuditEntry: AuditEntry{
		ID: "aud_pd_01", ActorName: "t", Action: "theme.update",
		TargetType: "theme", TargetID: "thm_pd_01", CreatedAt: now,
	}}
	if _, err := store.UpdateTheme(ctx, "thm_pd_01", ThemePatch{Default: &enable}, now, audit); !errors.Is(err, ErrPrivateDefault) {
		t.Fatalf("err = %v, want ErrPrivateDefault", err)
	}
}

func TestThemeListingIncludesOwnerAndStatus(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := themes.SyncBuiltin(ctx, themes.NewStore(db), time.Now().UTC()); err != nil {
		t.Fatalf("SyncBuiltin() error = %v", err)
	}
	// 造一个属于 alice 的私有主题 + 一个 active 当前版本。
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`INSERT INTO users (id, username, email, password_hash, role, status, created_at, updated_at)
		VALUES ('usr_alice_b2a', 'alice', 'alice@example.com', 'x', 'user', 'active', ?, ?)`, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO themes (id, name, version, author, description, mode, preview, enabled, is_default, created_at, updated_at, slug, scope, owner_id, source_type)
		VALUES ('thm_alice_b2a', 'Alice Theme', '1.0.0', 'alice', '', 'light', '', 1, 0, ?, ?, 'alice-theme', 'private', 'usr_alice_b2a', 'upload')`, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO theme_versions (id, theme_id, version, source_ref, manifest_json, compiled_css, content_hash, status, created_at)
		VALUES ('vaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'thm_alice_b2a', '1.0.0', 'digest', '{}', 'x', 'hashalicetheme0001', 'active', ?)`, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE themes SET current_version_id = 'vaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' WHERE id = 'thm_alice_b2a'`); err != nil {
		t.Fatal(err)
	}

	store := NewSQLStore(db)
	items, err := store.ListThemes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]Theme{}
	for _, item := range items {
		byID[item.ID] = item
	}
	alice, ok := byID["thm_alice_b2a"]
	if !ok {
		t.Fatal("列表缺少私有主题")
	}
	if alice.OwnerID != "usr_alice_b2a" || alice.OwnerName != "alice" {
		t.Fatalf("owner 未接线: %+v", alice)
	}
	if alice.Status != "active" {
		t.Fatalf("status = %q, want active", alice.Status)
	}
	// catalog 内置主题无 owner。
	if slate := byID["slate"]; slate.OwnerName != "" || slate.OwnerID != "" {
		t.Fatalf("catalog 主题不应有 owner: %+v", slate)
	}
	if slate := byID["slate"]; slate.Status != "active" {
		t.Fatalf("slate status = %q, want active", slate.Status)
	}
}

func TestUpdateThemeDisablesCurrentVersion(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := themes.SyncBuiltin(ctx, themes.NewStore(db), time.Now().UTC()); err != nil {
		t.Fatalf("SyncBuiltin() error = %v", err)
	}
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	insertTestUser(t, db, "usr_admin_b2a", "admin", "admin-b2a@example.com", "admin", now)
	store := NewSQLStore(db)
	service := NewService(store)
	service.now = func() time.Time { return now }
	actor := Actor{ID: "usr_admin_b2a", Username: "admin", Role: "admin", Status: "active"}

	// sakura 非默认，可停用。
	disabled := "disabled"
	updated, err := service.UpdateTheme(ctx, actor, "sakura", ThemePatch{Status: &disabled, RequestID: "req-1"})
	if err != nil {
		t.Fatalf("disable sakura version error = %v", err)
	}
	if updated.Status != "disabled" {
		t.Fatalf("status = %q, want disabled", updated.Status)
	}
	// 停用后 sakura 不再可解析，回落默认（slate）。
	var slateVersion string
	if err := db.QueryRow(`SELECT current_version_id FROM themes WHERE id = 'slate'`).Scan(&slateVersion); err != nil {
		t.Fatal(err)
	}
	got, err := themes.ResolveEligibleVersion(ctx, db, "sakura", "")
	if err != nil {
		t.Fatalf("resolve after disable error = %v", err)
	}
	if got != slateVersion {
		t.Fatalf("disabled theme should fall back to default %q, got %q", slateVersion, got)
	}
	// 重新启用恢复。
	active := "active"
	if _, err := service.UpdateTheme(ctx, actor, "sakura", ThemePatch{Status: &active, RequestID: "req-2"}); err != nil {
		t.Fatalf("re-enable error = %v", err)
	}
	reGot, err := themes.ResolveEligibleVersion(ctx, db, "sakura", "")
	if err != nil || reGot == slateVersion {
		t.Fatalf("re-enabled sakura should resolve to its own version, got %q (err %v)", reGot, err)
	}
}

func TestUpdateThemeRejectsDisablingDefaultVersion(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := themes.SyncBuiltin(ctx, themes.NewStore(db), time.Now().UTC()); err != nil {
		t.Fatalf("SyncBuiltin() error = %v", err)
	}
	service := NewService(NewSQLStore(db))
	service.now = func() time.Time { return time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC) }
	actor := Actor{ID: "usr_admin_b2a", Username: "admin", Role: "admin", Status: "active"}
	disabled := "disabled"
	// slate 是默认主题，拒绝停用其当前版本。
	if _, err := service.UpdateTheme(ctx, actor, "slate", ThemePatch{Status: &disabled, RequestID: "req"}); !errors.Is(err, ErrDefaultThemeVersion) {
		t.Fatalf("err = %v, want ErrDefaultThemeVersion", err)
	}
}
