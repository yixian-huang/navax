package httpapi

import (
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/yixian-huang/navax/internal/directoryadmin"
)

func TestDirectoryAdminHandlerLifecycle(t *testing.T) {
	db, authService, _, adminSession, token := setupHandlerServices(t)
	service := directoryadmin.NewService(directoryadmin.NewSQLStore(db))
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	NewDirectoryAdminHandler(authService, service).Mount(router)

	response := performRequest(router, http.MethodGet, "/admin/directory/categories", nil, "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d", response.Code)
	}
	response = performRequest(router, http.MethodPost, "/admin/directory/categories", map[string]any{"name": "工具", "icon": "tool"}, token)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing enabled status = %d", response.Code)
	}
	response = performRequest(router, http.MethodPost, "/admin/directory/categories", map[string]any{
		"name": "工具", "icon": "tool", "enabled": true,
	}, token)
	if response.Code != http.StatusCreated {
		t.Fatalf("create category status = %d: %s", response.Code, response.Body.String())
	}
	category := decodeEnvelope(t, response)["data"].(map[string]any)
	categoryID := category["id"].(string)

	response = performRequest(router, http.MethodPost, "/admin/directory/sites", map[string]any{
		"categoryId": categoryID, "title": "Go", "url": "https://go.dev", "icon": "", "description": "Go", "enabled": true,
	}, token)
	if response.Code != http.StatusCreated {
		t.Fatalf("create site status = %d: %s", response.Code, response.Body.String())
	}
	site := decodeEnvelope(t, response)["data"].(map[string]any)
	siteID := site["id"].(string)
	response = performRequest(router, http.MethodGet, "/admin/directory/sites?search=Go", nil, token)
	if response.Code != http.StatusOK {
		t.Fatalf("list sites status = %d: %s", response.Code, response.Body.String())
	}
	page := decodeEnvelope(t, response)
	if len(page["data"].([]any)) != 1 || page["meta"].(map[string]any)["total"].(float64) != 1 {
		t.Fatalf("site page = %+v", page)
	}
	response = performRequest(router, http.MethodDelete, "/admin/directory/categories/"+categoryID, nil, token)
	if response.Code != http.StatusConflict {
		t.Fatalf("delete non-empty category status = %d", response.Code)
	}
	response = performRequest(router, http.MethodPatch, "/admin/directory/sites/"+siteID, map[string]any{"enabled": false}, token)
	if response.Code != http.StatusOK || decodeEnvelope(t, response)["data"].(map[string]any)["enabled"] != false {
		t.Fatalf("disable site response = %d %s", response.Code, response.Body.String())
	}
	response = performRequest(router, http.MethodDelete, "/admin/directory/sites/"+siteID, nil, token)
	if response.Code != http.StatusOK {
		t.Fatalf("delete site status = %d", response.Code)
	}
	response = performRequest(router, http.MethodDelete, "/admin/directory/categories/"+categoryID, nil, token)
	if response.Code != http.StatusOK {
		t.Fatalf("delete category status = %d", response.Code)
	}

	var pageID, personalCategoryID string
	if err := db.QueryRow("SELECT id FROM navigation_pages WHERE owner_id = ? AND kind = 'personal'", adminSession.User.ID).Scan(&pageID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT id FROM categories WHERE page_id = ? AND is_uncategorized = 1", pageID).Scan(&personalCategoryID); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`
		INSERT INTO sites(id, page_id, category_id, title, url, sort_order, created_at, updated_at)
		VALUES ('site_http_admin_link', ?, ?, '待删除', 'https://example.com', 0, ?, ?)`, pageID, personalCategoryID, now, now); err != nil {
		t.Fatal(err)
	}
	response = performRequest(router, http.MethodGet, "/admin/links?ownerId="+adminSession.User.ID, nil, token)
	if response.Code != http.StatusOK || len(decodeEnvelope(t, response)["data"].([]any)) != 1 {
		t.Fatalf("admin links response = %d %s", response.Code, response.Body.String())
	}
	var beforeRevision int
	if err := db.QueryRow("SELECT draft_revision FROM navigation_pages WHERE id = ?", pageID).Scan(&beforeRevision); err != nil {
		t.Fatal(err)
	}
	response = performRequest(router, http.MethodDelete, "/admin/links/site_http_admin_link", map[string]any{"reason": "违规"}, token)
	if response.Code != http.StatusOK {
		t.Fatalf("delete link status = %d: %s", response.Code, response.Body.String())
	}
	var afterRevision int
	if err := db.QueryRow("SELECT draft_revision FROM navigation_pages WHERE id = ?", pageID).Scan(&afterRevision); err != nil {
		t.Fatal(err)
	}
	if afterRevision != beforeRevision+1 {
		t.Fatalf("draft revision before=%d after=%d", beforeRevision, afterRevision)
	}
	var requestID string
	if err := db.QueryRow("SELECT request_id FROM audit_logs WHERE action = 'link.admin_delete'").Scan(&requestID); err != nil {
		t.Fatal(err)
	}
	if requestID == "" {
		t.Fatal("admin link deletion did not persist request ID")
	}
}
