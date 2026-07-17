package navigation

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/yixian-huang/navax/internal/database"
)

func TestNavigationDraftPublicationIsolation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, service := testNavigationService(t)
	owner := insertTestPersonalPage(t, db, "user_owner_one", "owner-one", "page_owner_one", "category_owner_uncat", "owner-one")

	page, err := service.CurrentPage(ctx, owner, PageKindPersonal)
	if err != nil {
		t.Fatalf("CurrentPage() error = %v", err)
	}
	category, err := service.CreateCategory(ctx, owner, page.ID, CategoryInput{Name: "开发", Icon: "code"})
	if err != nil {
		t.Fatalf("CreateCategory() error = %v", err)
	}
	site, err := service.CreateSite(ctx, owner, page.ID, SiteInput{
		CategoryID: category.ID, Title: "Go", URL: "https://go.dev", Description: "Go 官网",
	})
	if err != nil {
		t.Fatalf("CreateSite() error = %v", err)
	}
	page, err = service.PageDraft(ctx, owner, page.ID)
	if err != nil {
		t.Fatalf("PageDraft() error = %v", err)
	}
	if page.DraftRevision != 2 || len(page.Categories) != 2 || len(page.Sites) != 1 {
		t.Fatalf("draft after create = revision %d, categories %d, sites %d", page.DraftRevision, len(page.Categories), len(page.Sites))
	}

	publication, err := service.Publish(ctx, owner, page.ID, page.DraftRevision, "https://nav.ax")
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if !publication.Published || publication.PublishedRevision == nil || *publication.PublishedRevision != 2 {
		t.Fatalf("publication = %#v", publication)
	}
	published, err := service.PublicBySlug(ctx, "owner-one")
	if err != nil {
		t.Fatalf("PublicBySlug() error = %v", err)
	}
	if published.Title != "owner-one 的导航" || len(published.Categories) != 2 || published.Categories[1].Sites[0].ID != site.ID {
		t.Fatalf("published payload = %#v", published)
	}
	firstSnapshotID := published.SnapshotID

	newTitle := "尚未发布的新标题"
	page, err = service.UpdatePage(ctx, owner, page.ID, PagePatch{ExpectedRevision: page.DraftRevision, Title: &newTitle})
	if err != nil {
		t.Fatalf("UpdatePage() error = %v", err)
	}
	_, err = service.ReplacePublication(ctx, owner, page.ID, PublicationSettingsInput{
		Visibility: VisibilityPublic, Slug: "owner-new", ShowAuthor: true,
	}, "https://nav.ax")
	if err != nil {
		t.Fatalf("ReplacePublication() error = %v", err)
	}

	stillPublished, err := service.PublicBySlug(ctx, "owner-one")
	if err != nil {
		t.Fatalf("old snapshot after draft changes error = %v", err)
	}
	if stillPublished.Title != "owner-one 的导航" || stillPublished.SnapshotID != firstSnapshotID || stillPublished.Visibility != VisibilityUnlisted {
		t.Fatalf("draft leaked into public snapshot: %#v", stillPublished)
	}
	if _, err := service.PublicBySlug(ctx, "owner-new"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("new unpublished slug error = %v, want ErrNotFound", err)
	}

	publication, err = service.Publish(ctx, owner, page.ID, page.DraftRevision, "https://nav.ax")
	if err != nil {
		t.Fatalf("second Publish() error = %v", err)
	}
	republished, err := service.PublicBySlug(ctx, "owner-new")
	if err != nil {
		t.Fatalf("new public snapshot error = %v", err)
	}
	if republished.Title != newTitle || republished.SnapshotID == firstSnapshotID || republished.Visibility != VisibilityPublic {
		t.Fatalf("republished payload = %#v", republished)
	}
	if _, err := service.PublicBySlug(ctx, "owner-one"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("old slug after republish error = %v, want ErrNotFound", err)
	}
	if _, err := db.ExecContext(ctx, "UPDATE users SET status = 'disabled' WHERE id = ?", owner.UserID); err != nil {
		t.Fatalf("disable owner: %v", err)
	}
	if _, err := service.PublicBySlug(ctx, "owner-new"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("disabled owner's public page error = %v, want ErrNotFound", err)
	}
	if _, err := db.ExecContext(ctx, "UPDATE users SET status = 'active' WHERE id = ?", owner.UserID); err != nil {
		t.Fatalf("enable owner: %v", err)
	}

	publication, err = service.Unpublish(ctx, owner, page.ID, "https://nav.ax")
	if err != nil {
		t.Fatalf("Unpublish() error = %v", err)
	}
	if publication.Published || publication.Visibility != VisibilityPrivate {
		t.Fatalf("unpublished publication = %#v", publication)
	}
	if _, err := service.Unpublish(ctx, owner, page.ID, "https://nav.ax"); err != nil {
		t.Fatalf("idempotent Unpublish() error = %v", err)
	}
	if _, err := service.PublicBySlug(ctx, "owner-new"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("public page after unpublish error = %v, want ErrNotFound", err)
	}
	var snapshotCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM published_snapshots WHERE page_id = ?", page.ID).Scan(&snapshotCount); err != nil {
		t.Fatalf("count immutable snapshots: %v", err)
	}
	if snapshotCount != 2 {
		t.Fatalf("snapshot count = %d, want 2", snapshotCount)
	}
}

func TestApprovedSubdomainResolvesPublishedPage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, service := testNavigationService(t)
	owner := insertTestPersonalPage(t, db, "user_subdomain_one", "subdomain-owner", "page_subdomain_one", "category_subdomain_uncat", "subdomain-owner")
	if _, err := db.ExecContext(ctx, "UPDATE system_settings SET root_domain = 'nav.ax', subdomains_enabled = 1 WHERE id = 1"); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.ExecContext(ctx, `
		INSERT INTO subdomain_requests(id, user_id, label, full_domain, status, applied_at, reviewed_at)
		VALUES ('subdomain_request_one', ?, 'alice', 'alice.nav.ax', 'approved', ?, ?)`, owner.UserID, now, now); err != nil {
		t.Fatal(err)
	}
	page, err := service.CurrentPage(ctx, owner, PageKindPersonal)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Publish(ctx, owner, page.ID, page.DraftRevision, "https://nav.ax"); err != nil {
		t.Fatal(err)
	}
	published, err := service.PublicHomeForHost(ctx, "ALICE.NAV.AX.")
	if err != nil {
		t.Fatal(err)
	}
	if published.ID != page.ID || published.Subdomain == nil || *published.Subdomain != "alice.nav.ax" {
		t.Fatalf("subdomain published page = %+v", published)
	}
	if _, err := service.PublicHomeForHost(ctx, "missing.nav.ax"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown subdomain error = %v", err)
	}

	// Revoke must clear subdomain on public projection without republish.
	if _, err := db.ExecContext(ctx, "UPDATE subdomain_requests SET status = 'revoked' WHERE id = 'subdomain_request_one'"); err != nil {
		t.Fatal(err)
	}
	bySlug, err := service.PublicBySlug(ctx, published.Slug)
	if err != nil {
		t.Fatalf("PublicBySlug after revoke error = %v", err)
	}
	if bySlug.Subdomain != nil {
		t.Fatalf("revoked subdomain still present on public page: %q", *bySlug.Subdomain)
	}
	if _, err := service.PublicHomeForHost(ctx, "alice.nav.ax"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revoked host should 404, got %v", err)
	}
}

func TestReservedPublicationSlugRejected(t *testing.T) {
	_, service := testNavigationService(t)
	if _, err := service.ReplacePublication(context.Background(), Actor{UserID: "missing"}, "missing", PublicationSettingsInput{
		Visibility: VisibilityUnlisted, Slug: "admin", ShowAuthor: true,
	}, "https://nav.ax"); !errors.Is(err, ErrValidation) {
		t.Fatalf("reserved slug error = %v", err)
	}
}

func TestNavigationContentOrderIsCompleteAtomicAndRevisionGuarded(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, service := testNavigationService(t)
	owner := insertTestPersonalPage(t, db, "user_order_one", "order-one", "page_order_one", "category_order_uncat", "order-one")
	page, _ := service.CurrentPage(ctx, owner, PageKindPersonal)
	first, err := service.CreateCategory(ctx, owner, page.ID, CategoryInput{Name: "第一类"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateCategory(ctx, owner, page.ID, CategoryInput{Name: "第二类"})
	if err != nil {
		t.Fatal(err)
	}
	one, err := service.CreateSite(ctx, owner, page.ID, SiteInput{CategoryID: first.ID, Title: "one", URL: "https://one.example"})
	if err != nil {
		t.Fatal(err)
	}
	two, err := service.CreateSite(ctx, owner, page.ID, SiteInput{CategoryID: first.ID, Title: "two", URL: "https://two.example"})
	if err != nil {
		t.Fatal(err)
	}
	page, _ = service.PageDraft(ctx, owner, page.ID)

	order := []CategoryOrder{
		{ID: second.ID, SiteIDs: []string{two.ID}},
		{ID: "category_order_uncat", SiteIDs: []string{}},
		{ID: first.ID, SiteIDs: []string{one.ID}},
	}
	newRevision, err := service.ReplaceContentOrder(ctx, owner, page.ID, page.DraftRevision, order)
	if err != nil {
		t.Fatalf("ReplaceContentOrder() error = %v", err)
	}
	if newRevision != page.DraftRevision+1 {
		t.Fatalf("new revision = %d, want %d", newRevision, page.DraftRevision+1)
	}
	reordered, _ := service.PageDraft(ctx, owner, page.ID)
	if reordered.Categories[0].ID != second.ID || reordered.Categories[2].ID != first.ID {
		t.Fatalf("category order = %#v", reordered.Categories)
	}
	siteByID := map[string]Site{reordered.Sites[0].ID: reordered.Sites[0], reordered.Sites[1].ID: reordered.Sites[1]}
	if siteByID[two.ID].CategoryID != second.ID || siteByID[one.ID].CategoryID != first.ID {
		t.Fatalf("site moves = %#v", reordered.Sites)
	}

	if _, err := service.ReplaceContentOrder(ctx, owner, page.ID, page.DraftRevision, order); !errors.Is(err, ErrPrecondition) {
		t.Fatalf("stale reorder error = %v, want ErrPrecondition", err)
	}
	invalid := []CategoryOrder{
		{ID: second.ID, SiteIDs: []string{two.ID}},
		{ID: "category_order_uncat", SiteIDs: []string{}},
		{ID: first.ID, SiteIDs: []string{}},
	}
	if _, err := service.ReplaceContentOrder(ctx, owner, page.ID, reordered.DraftRevision, invalid); !errors.Is(err, ErrInvalidOrder) {
		t.Fatalf("incomplete reorder error = %v, want ErrInvalidOrder", err)
	}
	afterInvalid, _ := service.PageDraft(ctx, owner, page.ID)
	if afterInvalid.DraftRevision != reordered.DraftRevision || afterInvalid.Categories[0].ID != second.ID {
		t.Fatalf("invalid reorder was not atomic: %#v", afterInvalid)
	}
}

func TestNavigationCategoryDeletionModes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, service := testNavigationService(t)
	owner := insertTestPersonalPage(t, db, "user_delete_one", "delete-one", "page_delete_one", "category_delete_uncat", "delete-one")
	page, _ := service.CurrentPage(ctx, owner, PageKindPersonal)
	moveCategory, _ := service.CreateCategory(ctx, owner, page.ID, CategoryInput{Name: "待移动"})
	deleteCategory, _ := service.CreateCategory(ctx, owner, page.ID, CategoryInput{Name: "待删除"})
	movedSite, _ := service.CreateSite(ctx, owner, page.ID, SiteInput{CategoryID: moveCategory.ID, Title: "move", URL: "https://move.example"})
	deletedSite, _ := service.CreateSite(ctx, owner, page.ID, SiteInput{CategoryID: deleteCategory.ID, Title: "delete", URL: "https://delete.example"})

	if err := service.DeleteCategory(ctx, owner, page.ID, moveCategory.ID, DeleteCategoryRejectIfNotEmpty); !errors.Is(err, ErrCategoryNotEmpty) {
		t.Fatalf("reject deletion error = %v, want ErrCategoryNotEmpty", err)
	}
	if err := service.DeleteCategory(ctx, owner, page.ID, moveCategory.ID, DeleteCategoryMoveSites); err != nil {
		t.Fatalf("move deletion error = %v", err)
	}
	moved, err := service.Sites(ctx, owner, page.ID, "category_delete_uncat", "")
	if err != nil || len(moved) != 1 || moved[0].ID != movedSite.ID {
		t.Fatalf("moved sites = %#v, error = %v", moved, err)
	}
	if err := service.DeleteCategory(ctx, owner, page.ID, deleteCategory.ID, DeleteCategoryDeleteSites); err != nil {
		t.Fatalf("cascade deletion error = %v", err)
	}
	if _, err := service.Sites(ctx, owner, page.ID, deleteCategory.ID, ""); err != nil {
		t.Fatalf("list deleted category sites error = %v", err)
	}
	if _, err := service.UpdateSite(ctx, owner, page.ID, deletedSite.ID, SitePatch{Title: stringPointer("gone")}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted site update error = %v, want ErrNotFound", err)
	}
	if err := service.DeleteCategory(ctx, owner, page.ID, "category_delete_uncat", DeleteCategoryDeleteSites); !errors.Is(err, ErrUncategorized) {
		t.Fatalf("uncategorized deletion error = %v, want ErrUncategorized", err)
	}
}

func TestNavigationAuthorizationAndSystemHome(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, service := testNavigationService(t)
	owner := insertTestPersonalPage(t, db, "user_auth_owner", "auth-owner", "page_auth_owner", "category_auth_uncat", "auth-owner")
	other := insertTestPersonalPage(t, db, "user_auth_other", "auth-other", "page_auth_other", "category_auth_other_uncat", "auth-other")
	admin := Actor{UserID: "user_admin_root", Username: "admin", Role: "admin"}
	insertTestUser(t, db, admin.UserID, admin.Username, "admin")

	if _, err := service.PageDraft(ctx, other, "page_auth_owner"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("other user's draft error = %v, want ErrForbidden", err)
	}
	if _, err := service.PageDraft(ctx, admin, "page_auth_owner"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("admin personal draft error = %v, want ErrForbidden", err)
	}
	if _, err := service.CurrentPage(ctx, owner, PageKindSystem); !errors.Is(err, ErrForbidden) {
		t.Fatalf("user system page error = %v, want ErrForbidden", err)
	}
	systemPage, err := service.CurrentPage(ctx, admin, PageKindSystem)
	if err != nil {
		t.Fatalf("admin system page error = %v", err)
	}
	_, err = service.ReplacePublication(ctx, admin, systemPage.ID, PublicationSettingsInput{
		Visibility: VisibilityPublic, Slug: "home", ShowAuthor: false,
	}, "https://nav.ax")
	if err != nil {
		t.Fatalf("configure system publication: %v", err)
	}
	if _, err := service.Publish(ctx, admin, systemPage.ID, systemPage.DraftRevision, "https://nav.ax"); err != nil {
		t.Fatalf("publish system page: %v", err)
	}
	home, err := service.PublicHome(ctx)
	if err != nil {
		t.Fatalf("PublicHome() error = %v", err)
	}
	if home.ID != systemPage.ID || home.Owner.Visible || home.Owner.Name != "" || home.Owner.AvatarURL != "" || home.Kind != PageKindSystem {
		t.Fatalf("public home = %#v", home)
	}
}

func TestNavigationSettingsValidationAndOptimisticLock(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, service := testNavigationService(t)
	owner := insertTestPersonalPage(t, db, "user_settings_one", "settings-one", "page_settings_one", "category_settings_uncat", "settings-one")
	page, _ := service.CurrentPage(ctx, owner, PageKindPersonal)
	settings := page.Settings
	settings.Layout.Template = "sidebar"
	settings.Layout.Columns = 3
	if _, err := service.ReplaceSettings(ctx, owner, page.ID, page.DraftRevision, settings); err != nil {
		t.Fatalf("ReplaceSettings() error = %v", err)
	}
	if _, err := service.ReplaceSettings(ctx, owner, page.ID, page.DraftRevision, settings); !errors.Is(err, ErrPrecondition) {
		t.Fatalf("stale settings error = %v, want ErrPrecondition", err)
	}
	settings.Layout.Columns = 0
	if _, err := service.ReplaceSettings(ctx, owner, page.ID, page.DraftRevision+1, settings); !errors.Is(err, ErrValidation) {
		t.Fatalf("invalid settings error = %v, want ErrValidation", err)
	}
}

func testNavigationService(t *testing.T) (*sql.DB, *Service) {
	t.Helper()
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: filepath.Join(t.TempDir(), "navigation.db")})
	if err != nil {
		t.Fatalf("OpenAndMigrate() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, NewService(NewSQLStore(db))
}

func insertTestPersonalPage(t *testing.T, db *sql.DB, userID, username, pageID, categoryID, slug string) Actor {
	t.Helper()
	insertTestUser(t, db, userID, username, "user")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`
		INSERT INTO navigation_pages(id, kind, owner_id, title, description, settings_json, draft_updated_at, created_at, updated_at)
		VALUES (?, 'personal', ?, ?, '', ?, ?, ?, ?)`,
		pageID, userID, username+" 的导航", DefaultSettingsJSON, now, now, now,
	); err != nil {
		t.Fatalf("insert personal page: %v", err)
	}
	if _, err := db.Exec("INSERT INTO page_publications(page_id, visibility, slug, show_author, updated_at) VALUES (?, 'unlisted', ?, 1, ?)", pageID, slug, now); err != nil {
		t.Fatalf("insert publication: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO categories(id, page_id, name, icon, sort_order, is_uncategorized, created_at, updated_at)
		VALUES (?, ?, '未分类', '', 0, 1, ?, ?)`, categoryID, pageID, now, now); err != nil {
		t.Fatalf("insert uncategorized category: %v", err)
	}
	return Actor{UserID: userID, Username: username, AvatarURL: "https://avatar.example/" + username, Role: "user"}
}

func insertTestUser(t *testing.T, db *sql.DB, userID, username, role string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`
		INSERT INTO users(id, username, email, password_hash, avatar_url, role, created_at, updated_at)
		VALUES (?, ?, ?, 'hash', ?, ?, ?, ?)`,
		userID, username, username+"@example.test", "https://avatar.example/"+username, role, now, now,
	); err != nil {
		t.Fatalf("insert user: %v", err)
	}
}

func stringPointer(value string) *string { return &value }
