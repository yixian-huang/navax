package directoryadmin

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/yixian-huang/navax/internal/database"
	"github.com/yixian-huang/navax/internal/navigation"
	"github.com/yixian-huang/navax/internal/security"
)

func TestDirectoryAndAdminLinkLifecycle(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	insertDirectoryUser(t, db, "usr_admin_dir", "owner", "owner@example.com", "admin", now)
	insertDirectoryUser(t, db, "usr_alice_dir", "alice", "alice@example.com", "user", now)
	insertPersonalNavigation(t, db, "usr_alice_dir", now)

	service := NewService(NewSQLStore(db))
	service.now = func() time.Time { return now }
	admin := Actor{ID: "usr_admin_dir", Username: "owner", Role: "admin", Status: "active"}
	if _, err := service.Categories(ctx, Actor{ID: "usr_alice_dir", Username: "alice", Role: "user", Status: "active"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("non-admin error = %v", err)
	}

	category, err := service.CreateCategory(ctx, admin, CategoryInput{Name: "开发工具", Icon: "code", Enabled: true}, "req-category")
	if err != nil {
		t.Fatal(err)
	}
	if category.SortOrder != 0 || !category.Enabled {
		t.Fatalf("category = %+v", category)
	}
	if _, err := service.CreateCategory(ctx, admin, CategoryInput{Name: "开发工具", Icon: "code", Enabled: true}, "req-duplicate"); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate category error = %v", err)
	}
	secondCategory, err := service.CreateCategory(ctx, admin, CategoryInput{Name: "搜索", Icon: "search", Enabled: true}, "req-category-two")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateSite(ctx, admin, SiteInput{CategoryID: category.ID, Title: "Bad", URL: "javascript:alert(1)", Enabled: true}, "req-bad"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid URL error = %v", err)
	}
	site, err := service.CreateSite(ctx, admin, SiteInput{
		CategoryID: category.ID, Title: "Go", URL: "https://go.dev/", Description: "Go 官网", Enabled: true,
	}, "req-site")
	if err != nil {
		t.Fatal(err)
	}
	if site.CategoryName != category.Name || site.URL != "https://go.dev/" {
		t.Fatalf("site = %+v", site)
	}
	if err := service.DeleteCategory(ctx, admin, category.ID, "req-delete-nonempty"); !errors.Is(err, ErrCategoryInUse) {
		t.Fatalf("non-empty category error = %v", err)
	}
	newTitle := "Golang"
	updated, err := service.UpdateSite(ctx, admin, site.ID, SitePatch{CategoryID: &secondCategory.ID, Title: &newTitle}, "req-update-site")
	if err != nil {
		t.Fatal(err)
	}
	if updated.CategoryID != secondCategory.ID || updated.Title != newTitle {
		t.Fatalf("updated site = %+v", updated)
	}
	page, err := service.Sites(ctx, admin, SiteFilter{Search: "Golang", Page: 1, PageSize: 20})
	if err != nil || page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("site page = %+v, %v", page, err)
	}
	if err := service.DeleteSite(ctx, admin, site.ID, "req-delete-site"); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteCategory(ctx, admin, category.ID, "req-delete-category"); err != nil {
		t.Fatal(err)
	}

	links, err := service.Links(ctx, admin, LinkFilter{OwnerID: "usr_alice_dir", Page: 1, PageSize: 20})
	if err != nil || links.Total != 1 || links.Items[0].ID != "site_alice_link" {
		t.Fatalf("admin links = %+v, %v", links, err)
	}
	if err := service.DeleteLink(ctx, admin, "site_alice_link", "违规链接", "req-delete-link"); err != nil {
		t.Fatal(err)
	}
	var revision, siteCount int
	if err := db.QueryRowContext(ctx, "SELECT draft_revision FROM navigation_pages WHERE id = 'page_alice_dir'").Scan(&revision); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sites WHERE id = 'site_alice_link'").Scan(&siteCount); err != nil {
		t.Fatal(err)
	}
	if revision != 1 || siteCount != 0 {
		t.Fatalf("revision=%d siteCount=%d", revision, siteCount)
	}
	var detail, requestID string
	if err := db.QueryRowContext(ctx, "SELECT detail_json, request_id FROM audit_logs WHERE action = 'link.admin_delete'").Scan(&detail, &requestID); err != nil {
		t.Fatal(err)
	}
	if detail != `{"reason":"违规链接"}` || requestID != "req-delete-link" {
		t.Fatalf("audit detail=%q requestID=%q", detail, requestID)
	}
}

func insertDirectoryUser(t *testing.T, db *sql.DB, id, username, email, role string, now time.Time) {
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

func insertPersonalNavigation(t *testing.T, db *sql.DB, userID string, now time.Time) {
	t.Helper()
	timestamp := dbTime(now)
	_, err := db.Exec(`
		INSERT INTO navigation_pages(id, kind, owner_id, title, settings_json, draft_updated_at, created_at, updated_at)
		VALUES ('page_alice_dir', 'personal', ?, 'Alice', ?, ?, ?, ?)`, userID, navigation.DefaultSettingsJSON, timestamp, timestamp, timestamp)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		INSERT INTO categories(id, page_id, name, sort_order, is_uncategorized, created_at, updated_at)
		VALUES ('category_alice_dir', 'page_alice_dir', '未分类', 0, 1, ?, ?)`, timestamp, timestamp)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		INSERT INTO sites(id, page_id, category_id, title, url, sort_order, created_at, updated_at)
		VALUES ('site_alice_link', 'page_alice_dir', 'category_alice_dir', '示例', 'https://example.com', 0, ?, ?)`, timestamp, timestamp)
	if err != nil {
		t.Fatal(err)
	}
}
