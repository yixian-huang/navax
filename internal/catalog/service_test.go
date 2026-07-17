package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/yixian-huang/navax/internal/database"
	"github.com/yixian-huang/navax/internal/navigation"
	"github.com/yixian-huang/navax/internal/security"
)

func newTestService(t *testing.T) (*Service, *sql.DB) {
	t.Helper()
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatalf("OpenAndMigrate() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewService(db), db
}

func dbTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

func TestConfigReadsSeededSettings(t *testing.T) {
	service, _ := newTestService(t)

	config, err := service.Config(context.Background())
	if err != nil {
		t.Fatalf("Config() error = %v", err)
	}
	if config.InstanceName == "" {
		t.Fatalf("Config() 缺少默认实例名: %+v", config)
	}
	if config.RegistrationMode != "invite" && config.RegistrationMode != "closed" {
		t.Fatalf("registrationMode = %q", config.RegistrationMode)
	}
	if config.Limits.MaxUploadBytes <= 0 || config.Limits.MaxSitesPerPage <= 0 {
		t.Fatalf("limits 应为正数: %+v", config.Limits)
	}
}

func TestThemesReturnsEnabledSeeded(t *testing.T) {
	service, db := newTestService(t)

	themes, err := service.Themes(context.Background())
	if err != nil {
		t.Fatalf("Themes() error = %v", err)
	}
	if len(themes) == 0 {
		t.Fatal("Themes() 应返回已启用的种子主题")
	}
	if !themes[0].Default {
		t.Fatalf("默认主题应排在首位, got %+v", themes[0])
	}

	// 停用一个主题后不应再出现。
	if _, err := db.Exec("UPDATE themes SET enabled = 0 WHERE id = 'kyoto'"); err != nil {
		t.Fatal(err)
	}
	after, err := service.Themes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(themes)-1 {
		t.Fatalf("停用后主题数应减一: before=%d after=%d", len(themes), len(after))
	}
	for _, theme := range after {
		if theme.ID == "kyoto" {
			t.Fatal("已停用的 kyoto 不应出现")
		}
	}
}

func TestDirectorySearchFilterPaginate(t *testing.T) {
	service, db := newTestService(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)

	insertDirectoryCategory(t, db, "dcat_dev", "开发", 0, now)
	insertDirectoryCategory(t, db, "dcat_design", "设计", 1, now)
	insertDirectoryCategory(t, db, "dcat_off", "停用分类", 2, now)
	insertDirectorySite(t, db, "dsite_gh", "dcat_dev", "GitHub", "https://github.com", "代码托管", 0, true, now)
	insertDirectorySite(t, db, "dsite_go", "dcat_dev", "Go", "https://go.dev", "Go 官网", 1, true, now)
	insertDirectorySite(t, db, "dsite_fig", "dcat_design", "Figma", "https://figma.com", "设计工具", 0, true, now)
	// 停用的站点与停用分类下的站点都不应出现。
	insertDirectorySite(t, db, "dsite_hidden", "dcat_dev", "Hidden", "https://hidden.example", "隐藏", 2, false, now)
	insertDirectorySite(t, db, "dsite_offcat", "dcat_off", "OffCat", "https://offcat.example", "停用分类下", 0, true, now)

	all, err := service.Directory(ctx, "", "", 1, 20)
	if err != nil {
		t.Fatalf("Directory() error = %v", err)
	}
	if all.Total != 3 {
		t.Fatalf("启用站点数 = %d, want 3 (disabled/off-category excluded)", all.Total)
	}

	byCategory, err := service.Directory(ctx, "", "dcat_dev", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if byCategory.Total != 2 {
		t.Fatalf("开发分类站点数 = %d, want 2", byCategory.Total)
	}

	bySearch, err := service.Directory(ctx, "figma", "", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if bySearch.Total != 1 || bySearch.Items[0].Title != "Figma" {
		t.Fatalf("搜索 figma = %+v", bySearch)
	}
	if bySearch.Items[0].CategoryName != "设计" {
		t.Fatalf("站点应带分类名, got %q", bySearch.Items[0].CategoryName)
	}

	firstPage, err := service.Directory(ctx, "", "", 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstPage.Items) != 2 || firstPage.Total != 3 {
		t.Fatalf("分页第一页 = %+v", firstPage)
	}
	secondPage, err := service.Directory(ctx, "", "", 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(secondPage.Items) != 1 {
		t.Fatalf("分页第二页应剩 1 条, got %d", len(secondPage.Items))
	}
}

func TestDiscoverVisibilityAndFilters(t *testing.T) {
	service, db := newTestService(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)

	insertUser(t, db, "usr_pub0", "publisher", "pub@example.com", "user", "active", now)
	insertUser(t, db, "usr_priv", "private", "priv@example.com", "user", "active", now)
	insertUser(t, db, "usr_disabled", "disabled", "dis@example.com", "user", "disabled", now)

	// 公开且已发布 → 应出现
	insertPublishedPage(t, db, publishSeed{
		userID: "usr_pub0", pageID: "page_pub", slug: "alpha", title: "Alpha 导航",
		description: "开发者的导航", visibility: "public", tags: []string{"dev", "tools"},
		featured: true, now: now,
	})
	// 私有 → 不出现
	insertPublishedPage(t, db, publishSeed{
		userID: "usr_priv", pageID: "page_priv", slug: "beta", title: "Beta",
		description: "私有页", visibility: "unlisted", tags: nil, now: now,
	})
	// 账号停用 → 不出现
	insertPublishedPage(t, db, publishSeed{
		userID: "usr_disabled", pageID: "page_dis", slug: "gamma", title: "Gamma",
		description: "停用作者", visibility: "public", tags: nil, now: now,
	})

	all, err := service.Discover(ctx, "", "", "latest", 1, 20)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if all.Total != 1 || all.Items[0].Slug != "alpha" {
		t.Fatalf("Discover 只应含公开且账号活跃的页面, got %+v", all)
	}
	if all.Items[0].ThemeID == "" || all.Items[0].OwnerName != "publisher" {
		t.Fatalf("Discover 项应从快照解析主题与作者, got %+v", all.Items[0])
	}

	byTag, err := service.Discover(ctx, "", "tools", "latest", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if byTag.Total != 1 {
		t.Fatalf("按 tag=tools 过滤 = %d, want 1", byTag.Total)
	}

	missTag, err := service.Discover(ctx, "", "nonexistent", "latest", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if missTag.Total != 0 {
		t.Fatalf("不存在的 tag 应为空, got %d", missTag.Total)
	}

	bySearch, err := service.Discover(ctx, "Alpha", "", "latest", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if bySearch.Total != 1 {
		t.Fatalf("按标题搜索 = %d, want 1", bySearch.Total)
	}

	if _, err := db.ExecContext(ctx, "UPDATE system_settings SET discover_enabled = 0 WHERE id = 1"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Discover(ctx, "", "", "latest", 1, 20); !errors.Is(err, ErrDiscoverDisabled) {
		t.Fatalf("Discover when disabled error = %v, want ErrDiscoverDisabled", err)
	}
}

// ---- seed helpers ----

func insertUser(t *testing.T, db *sql.DB, id, username, email, role, status string, now time.Time) {
	t.Helper()
	hash, err := security.HashPassword("catalog-integration-pw")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		INSERT INTO users(id, username, email, password_hash, role, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, id, username, email, hash, role, status, dbTime(now), dbTime(now))
	if err != nil {
		t.Fatal(err)
	}
}

func insertDirectoryCategory(t *testing.T, db *sql.DB, id, name string, sortOrder int, now time.Time) {
	t.Helper()
	enabled := 1
	if name == "停用分类" {
		enabled = 0
	}
	_, err := db.Exec(`
		INSERT INTO directory_categories(id, name, icon, sort_order, enabled, created_at, updated_at)
		VALUES (?, ?, '', ?, ?, ?, ?)`, id, name, sortOrder, enabled, dbTime(now), dbTime(now))
	if err != nil {
		t.Fatal(err)
	}
}

func insertDirectorySite(t *testing.T, db *sql.DB, id, categoryID, title, url, description string, sortOrder int, enabled bool, now time.Time) {
	t.Helper()
	enabledValue := 0
	if enabled {
		enabledValue = 1
	}
	_, err := db.Exec(`
		INSERT INTO directory_sites(id, category_id, title, url, icon, description, sort_order, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, '', ?, ?, ?, ?, ?)`,
		id, categoryID, title, url, description, sortOrder, enabledValue, dbTime(now), dbTime(now))
	if err != nil {
		t.Fatal(err)
	}
}

type publishSeed struct {
	userID, pageID, slug, title, description, visibility string
	tags                                                 []string
	featured                                             bool
	now                                                  time.Time
}

func insertPublishedPage(t *testing.T, db *sql.DB, seed publishSeed) {
	t.Helper()
	timestamp := dbTime(seed.now)
	if _, err := db.Exec(`
		INSERT INTO navigation_pages(id, kind, owner_id, title, settings_json, draft_updated_at, created_at, updated_at)
		VALUES (?, 'personal', ?, ?, ?, ?, ?, ?)`,
		seed.pageID, seed.userID, seed.title, navigation.DefaultSettingsJSON, timestamp, timestamp, timestamp); err != nil {
		t.Fatal(err)
	}

	payload := navigation.PublishedPage{
		ID:          seed.pageID,
		SnapshotID:  "snap_" + seed.pageID,
		Kind:        navigation.PageKind("personal"),
		Title:       seed.title,
		Description: seed.description,
		Slug:        seed.slug,
		Visibility:  navigation.Visibility(seed.visibility),
		Owner:       navigation.PublishedOwner{Name: mustUsername(t, db, seed.userID)},
		Settings:    navigation.PageSettings{Appearance: navigation.AppearanceSettings{ThemeID: "slate"}},
		PublishedAt: seed.now,
		ETag:        "etag_" + seed.pageID,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	snapshotID := "snap_" + seed.pageID
	if _, err := db.Exec(`
		INSERT INTO published_snapshots(id, page_id, draft_revision, slug, visibility, payload_json, etag, published_at)
		VALUES (?, ?, 1, ?, ?, ?, ?, ?)`,
		snapshotID, seed.pageID, seed.slug, seed.visibility, string(payloadJSON), "etag_"+seed.pageID, timestamp); err != nil {
		t.Fatal(err)
	}

	tagsJSON := "[]"
	if len(seed.tags) > 0 {
		encoded, err := json.Marshal(seed.tags)
		if err != nil {
			t.Fatal(err)
		}
		tagsJSON = string(encoded)
	}
	featured := 0
	if seed.featured {
		featured = 1
	}
	if _, err := db.Exec(`
		INSERT INTO page_publications(page_id, visibility, slug, current_snapshot_id, featured, tags_json, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		seed.pageID, seed.visibility, seed.slug, snapshotID, featured, tagsJSON, timestamp); err != nil {
		t.Fatal(err)
	}
}

func mustUsername(t *testing.T, db *sql.DB, userID string) string {
	t.Helper()
	var username string
	if err := db.QueryRow("SELECT username FROM users WHERE id = ?", userID).Scan(&username); err != nil {
		t.Fatal(err)
	}
	return username
}
