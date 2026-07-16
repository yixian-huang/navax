package dataexchange

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	navaxdb "github.com/yixian-huang/navax/internal/database"
	"github.com/yixian-huang/navax/internal/navigation"
	"github.com/yixian-huang/navax/internal/security"
)

const (
	testUserID = "user_import_owner"
	testPageID = "page_import_personal"
	testUncat  = "category_import_uncategorized"
	testCat    = "category_import_existing"
)

var testNow = time.Date(2026, 7, 16, 8, 30, 0, 0, time.UTC)

func TestPreviewAndMergeCommitAreIdempotent(t *testing.T) {
	db := openTestDB(t)
	seedPage(t, db, true)
	service := NewService(db)
	service.now = func() time.Time { return testNow }
	actor := testActor()

	content := []byte(`<!DOCTYPE NETSCAPE-Bookmark-file-1>
<DL><p>
  <DT><H3>开发</H3>
  <DL><p>
    <DT><A HREF="https://existing.example/">已有</A>
    <DT><A HREF="https://new.example/docs">新站点</A>
    <DT><A HREF="https://new.example/docs">重复的新站点</A>
    <DT><A HREF="javascript:alert(1)">无效</A>
  </DL><p>
</DL><p>`)
	preview, err := service.Preview(context.Background(), actor, testPageID, FormatBookmarksHTML, content)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Totals != (PreviewTotals{Categories: 1, Sites: 4, Duplicates: 2, Invalid: 1}) {
		t.Fatalf("unexpected totals: %+v", preview.Totals)
	}
	if preview.ExpiresAt != testNow.Add(defaultPreviewTTL) {
		t.Fatalf("unexpected expiry: %s", preview.ExpiresAt)
	}
	var storedToken string
	if err := db.QueryRow("SELECT CAST(token_hash AS TEXT) FROM import_previews WHERE page_id = ?", testPageID).Scan(&storedToken); err != nil {
		t.Fatal(err)
	}
	if storedToken == preview.ImportToken || storedToken != security.HashToken(preview.ImportToken) {
		t.Fatalf("preview token was not stored as a hash: %q", storedToken)
	}

	selected := previewSiteIDs(preview)
	input := CommitInput{ImportToken: preview.ImportToken, Mode: ModeMerge, SelectedSiteIDs: selected, ExpectedRevision: 0}
	result, err := service.Commit(context.Background(), actor, testPageID, "import-key-00000001", input)
	if err != nil {
		t.Fatal(err)
	}
	want := ImportResult{CategoriesCreated: 1, SitesCreated: 1, DuplicatesSkipped: 2, InvalidSkipped: 1, DraftRevision: 1}
	if result != want {
		t.Fatalf("result = %+v, want %+v", result, want)
	}

	retry, err := service.Commit(context.Background(), actor, testPageID, "import-key-00000001", input)
	if err != nil {
		t.Fatal(err)
	}
	if retry != want {
		t.Fatalf("idempotent retry = %+v, want %+v", retry, want)
	}
	assertIntQuery(t, db, 2, "SELECT COUNT(*) FROM sites WHERE page_id = ?", testPageID)
	assertIntQuery(t, db, 1, "SELECT draft_revision FROM navigation_pages WHERE id = ?", testPageID)
	assertIntQuery(t, db, 0, "SELECT COUNT(*) FROM import_previews WHERE page_id = ?", testPageID)

	changed := input
	changed.Mode = ModeReplace
	if _, err := service.Commit(context.Background(), actor, testPageID, "import-key-00000001", changed); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed idempotent request error = %v, want conflict", err)
	}
}

func TestReplaceRebuildsContentAndReevaluatesExistingDuplicates(t *testing.T) {
	db := openTestDB(t)
	seedPage(t, db, true)
	service := NewService(db)
	service.now = func() time.Time { return testNow }

	document := map[string]any{
		"format": "navax-export", "version": 1,
		"page": map[string]any{
			"categories": []map[string]any{{"id": "source-category", "name": "替换分类"}},
			"sites": []map[string]any{{
				"id": "source-site", "categoryId": "source-category", "title": "替换后的站点", "url": "https://existing.example/",
			}},
		},
	}
	content, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := service.Preview(context.Background(), testActor(), testPageID, FormatNavaxJSON, content)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Categories[0].Sites[0].Duplicate {
		t.Fatal("existing URL should be marked duplicate during preview")
	}
	result, err := service.Commit(context.Background(), testActor(), testPageID, "replace-key-000001", CommitInput{
		ImportToken: preview.ImportToken, Mode: ModeReplace,
		SelectedSiteIDs: []string{"source-site"}, ExpectedRevision: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != (ImportResult{CategoriesCreated: 1, SitesCreated: 1, DraftRevision: 1}) {
		t.Fatalf("unexpected replace result: %+v", result)
	}
	assertIntQuery(t, db, 1, "SELECT COUNT(*) FROM sites WHERE page_id = ? AND title = '替换后的站点'", testPageID)
	assertIntQuery(t, db, 0, "SELECT COUNT(*) FROM sites WHERE page_id = ? AND title = '已有站点'", testPageID)
	assertIntQuery(t, db, 1, "SELECT COUNT(*) FROM categories WHERE page_id = ? AND is_uncategorized = 1", testPageID)
}

func TestCommitLimitFailureRollsBackAllChanges(t *testing.T) {
	db := openTestDB(t)
	seedPage(t, db, true)
	if _, err := db.Exec("UPDATE system_settings SET max_sites_per_page = 1 WHERE id = 1"); err != nil {
		t.Fatal(err)
	}
	service := NewService(db)
	service.now = func() time.Time { return testNow }
	preview, err := service.Preview(context.Background(), testActor(), testPageID, FormatBookmarksHTML, []byte(`
<DL><p><DT><H3>新分类</H3>
<DL><p><DT><A HREF="https://over-limit.example/">超限站点</A>
</DL><p></DL><p>`))
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Commit(context.Background(), testActor(), testPageID, "rollback-key-00001", CommitInput{
		ImportToken: preview.ImportToken, Mode: ModeMerge,
		SelectedSiteIDs: previewSiteIDs(preview), ExpectedRevision: 0,
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("commit error = %v, want conflict", err)
	}
	assertIntQuery(t, db, 1, "SELECT COUNT(*) FROM sites WHERE page_id = ?", testPageID)
	assertIntQuery(t, db, 2, "SELECT COUNT(*) FROM categories WHERE page_id = ?", testPageID)
	assertIntQuery(t, db, 0, "SELECT draft_revision FROM navigation_pages WHERE id = ?", testPageID)
	assertIntQuery(t, db, 1, "SELECT COUNT(*) FROM import_previews WHERE page_id = ?", testPageID)
	assertIntQuery(t, db, 0, "SELECT COUNT(*) FROM idempotency_records WHERE scope = ?", "page-import:"+testPageID)
}

func TestRevisionMismatchPreservesPreviewAndContent(t *testing.T) {
	db := openTestDB(t)
	seedPage(t, db, true)
	service := NewService(db)
	preview, err := service.Preview(context.Background(), testActor(), testPageID, FormatBookmarksHTML, []byte(`
<DL><p><DT><A HREF="https://new.example/">新站点</A></DL><p>`))
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Commit(context.Background(), testActor(), testPageID, "revision-key-00001", CommitInput{
		ImportToken: preview.ImportToken, Mode: ModeMerge,
		SelectedSiteIDs: previewSiteIDs(preview), ExpectedRevision: 4,
	})
	if !errors.Is(err, navigation.ErrPrecondition) {
		t.Fatalf("commit error = %v, want precondition", err)
	}
	assertIntQuery(t, db, 1, "SELECT COUNT(*) FROM sites WHERE page_id = ?", testPageID)
	assertIntQuery(t, db, 1, "SELECT COUNT(*) FROM import_previews WHERE page_id = ?", testPageID)
}

func TestExpiredPreviewCannotCommit(t *testing.T) {
	db := openTestDB(t)
	seedPage(t, db, true)
	service := NewService(db)
	service.now = func() time.Time { return testNow }
	preview, err := service.Preview(context.Background(), testActor(), testPageID, FormatBookmarksHTML, []byte(`
<!DOCTYPE NETSCAPE-Bookmark-file-1>
<DL><p><DT><A HREF="https://new.example/">新站点</A></DL><p>`))
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return testNow.Add(defaultPreviewTTL) }
	_, err = service.Commit(context.Background(), testActor(), testPageID, "expired-key-000001", CommitInput{
		ImportToken: preview.ImportToken, Mode: ModeMerge,
		SelectedSiteIDs: previewSiteIDs(preview), ExpectedRevision: 0,
	})
	if !errors.Is(err, ErrImportExpired) {
		t.Fatalf("commit error = %v, want expired preview", err)
	}
	assertIntQuery(t, db, 1, "SELECT COUNT(*) FROM sites WHERE page_id = ?", testPageID)
	assertIntQuery(t, db, 0, "SELECT draft_revision FROM navigation_pages WHERE id = ?", testPageID)
}

func TestEmptyNavaxReplaceClearsPageAtomically(t *testing.T) {
	db := openTestDB(t)
	seedPage(t, db, true)
	service := NewService(db)
	document := []byte(`{"format":"navax-export","version":1,"page":{"categories":[],"sites":[]}}`)
	preview, err := service.Preview(context.Background(), testActor(), testPageID, FormatNavaxJSON, document)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Commit(context.Background(), testActor(), testPageID, "empty-key-00000001", CommitInput{
		ImportToken: preview.ImportToken, Mode: ModeReplace, SelectedSiteIDs: []string{}, ExpectedRevision: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != (ImportResult{DraftRevision: 1}) {
		t.Fatalf("unexpected empty replace result: %+v", result)
	}
	assertIntQuery(t, db, 0, "SELECT COUNT(*) FROM sites WHERE page_id = ?", testPageID)
	assertIntQuery(t, db, 1, "SELECT COUNT(*) FROM categories WHERE page_id = ?", testPageID)
}

func TestExportProducesPortableJSONAndBookmarksHTML(t *testing.T) {
	db := openTestDB(t)
	seedPage(t, db, true)
	service := NewService(db)
	service.now = func() time.Time { return testNow }

	jsonFile, err := service.Export(context.Background(), testActor(), testPageID, FormatNavaxJSON)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(jsonFile.ContentType, "application/json") || !strings.HasSuffix(jsonFile.Filename, ".json") {
		t.Fatalf("unexpected JSON metadata: %+v", jsonFile)
	}
	var document PortableExport
	if err := json.Unmarshal(jsonFile.Content, &document); err != nil {
		t.Fatal(err)
	}
	if document.Format != "navax-export" || document.Version != 1 || document.Page.ID != testPageID || len(document.Page.Sites) != 1 {
		t.Fatalf("unexpected portable export: %+v", document)
	}

	htmlFile, err := service.Export(context.Background(), testActor(), testPageID, FormatBookmarksHTML)
	if err != nil {
		t.Fatal(err)
	}
	body := string(htmlFile.Content)
	if !strings.Contains(body, "<!DOCTYPE NETSCAPE-Bookmark-file-1>") ||
		!strings.Contains(body, `HREF="https://existing.example/"`) || !strings.Contains(body, "已有站点") {
		t.Fatalf("unexpected bookmark export: %s", body)
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := navaxdb.OpenAndMigrate(context.Background(), navaxdb.Config{Path: t.TempDir() + "/dataexchange.sqlite"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func seedPage(t *testing.T, db *sql.DB, withSite bool) {
	t.Helper()
	now := testNow.Format(time.RFC3339Nano)
	if _, err := db.Exec(`
		INSERT INTO users(id, username, email, password_hash, role, created_at, updated_at)
		VALUES (?, 'importer', 'importer@example.test', 'not-used', 'user', ?, ?)`, testUserID, now, now); err != nil {
		t.Fatal(err)
	}
	var settings string
	if err := db.QueryRow("SELECT settings_json FROM navigation_pages WHERE id = 'page_system_root'").Scan(&settings); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO navigation_pages(
			id, kind, owner_id, title, description, draft_revision, settings_json, draft_updated_at, created_at, updated_at
		) VALUES (?, 'personal', ?, '我的导航', '', 0, ?, ?, ?, ?)`, testPageID, testUserID, settings, now, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO page_publications(page_id, visibility, slug, show_author, updated_at)
		VALUES (?, 'private', 'import-test', 1, ?)`, testPageID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO categories(id, page_id, name, icon, sort_order, is_uncategorized, created_at, updated_at)
		VALUES (?, ?, '未分类', '', 0, 1, ?, ?), (?, ?, '已有分类', '', 1, 0, ?, ?)`,
		testUncat, testPageID, now, now, testCat, testPageID, now, now); err != nil {
		t.Fatal(err)
	}
	if withSite {
		if _, err := db.Exec(`
			INSERT INTO sites(id, page_id, category_id, title, url, icon, description, sort_order, created_at, updated_at)
			VALUES ('site_import_existing', ?, ?, '已有站点', 'https://existing.example/', '', '', 0, ?, ?)`, testPageID, testCat, now, now); err != nil {
			t.Fatal(err)
		}
	}
}

func testActor() navigation.Actor {
	return navigation.Actor{UserID: testUserID, Username: "importer", Role: "user"}
}

func previewSiteIDs(preview Preview) []string {
	var result []string
	for _, category := range preview.Categories {
		for _, site := range category.Sites {
			result = append(result, site.SourceID)
		}
	}
	return result
}

func assertIntQuery(t *testing.T, db *sql.DB, want int, query string, args ...any) {
	t.Helper()
	var got int
	if err := db.QueryRow(query, args...).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("query %q = %d, want %d", query, got, want)
	}
}
