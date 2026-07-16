package httpapi

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/yixian-huang/navax/internal/subdomains"
)

func TestSubdomainHandlerLifecycle(t *testing.T) {
	db, authService, _, _, token := setupHandlerServices(t)
	if _, err := db.Exec("UPDATE system_settings SET subdomains_enabled = 1, root_domain = 'nav.ax'"); err != nil {
		t.Fatal(err)
	}
	service := subdomains.NewService(subdomains.NewSQLStore(db))
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	NewSubdomainHandler(authService, service).Mount(router)

	response := performRequest(router, http.MethodGet, "/me/subdomain", nil, "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d", response.Code)
	}
	response = performRequest(router, http.MethodGet, "/me/subdomain", nil, token)
	if response.Code != http.StatusOK || decodeEnvelope(t, response)["data"] != nil {
		t.Fatalf("empty subdomain response = %d %s", response.Code, response.Body.String())
	}
	response = performRequest(router, http.MethodPost, "/me/subdomain", map[string]any{"label": "admin"}, token)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("reserved label status = %d", response.Code)
	}
	response = performRequest(router, http.MethodPost, "/me/subdomain", map[string]any{"label": "own"}, token)
	if response.Code != http.StatusCreated {
		t.Fatalf("apply status = %d: %s", response.Code, response.Body.String())
	}
	created := decodeEnvelope(t, response)["data"].(map[string]any)
	requestID := created["id"].(string)
	if created["fullDomain"] != "own.nav.ax" || created["status"] != "pending" {
		t.Fatalf("created request = %+v", created)
	}

	response = performRequest(router, http.MethodGet, "/admin/subdomains?status=pending&pageSize=10", nil, token)
	if response.Code != http.StatusOK {
		t.Fatalf("admin list status = %d: %s", response.Code, response.Body.String())
	}
	listed := decodeEnvelope(t, response)
	if len(listed["data"].([]any)) != 1 || listed["meta"].(map[string]any)["total"].(float64) != 1 {
		t.Fatalf("admin list = %+v", listed)
	}
	response = performRequest(router, http.MethodPatch, "/admin/subdomains/"+requestID, map[string]any{"decision": "approve"}, token)
	if response.Code != http.StatusOK {
		t.Fatalf("approve status = %d: %s", response.Code, response.Body.String())
	}
	approved := decodeEnvelope(t, response)["data"].(map[string]any)
	if approved["status"] != "approved" || approved["reviewedAt"] == nil {
		t.Fatalf("approved request = %+v", approved)
	}
	response = performRequest(router, http.MethodDelete, "/me/subdomain", nil, token)
	if response.Code != http.StatusConflict {
		t.Fatalf("user cancel approved status = %d", response.Code)
	}
	response = performRequest(router, http.MethodPatch, "/admin/subdomains/"+requestID, map[string]any{
		"decision": "revoke", "reason": "管理员撤销",
	}, token)
	if response.Code != http.StatusOK {
		t.Fatalf("revoke status = %d: %s", response.Code, response.Body.String())
	}
	revoked := decodeEnvelope(t, response)["data"].(map[string]any)
	if revoked["status"] != "revoked" || revoked["reason"] != "管理员撤销" {
		t.Fatalf("revoked request = %+v", revoked)
	}

	response = performRequest(router, http.MethodPost, "/me/subdomain", map[string]any{"label": "owner-nav"}, token)
	if response.Code != http.StatusCreated {
		t.Fatalf("auto-approve status = %d: %s", response.Code, response.Body.String())
	}
	automaticallyApproved := decodeEnvelope(t, response)["data"].(map[string]any)
	if automaticallyApproved["fullDomain"] != "owner-nav.nav.ax" || automaticallyApproved["status"] != "approved" || automaticallyApproved["reviewedAt"] == nil {
		t.Fatalf("automatically approved request = %+v", automaticallyApproved)
	}

	var auditRequestID string
	if err := db.QueryRow("SELECT request_id FROM audit_logs WHERE action = 'subdomain.approve'").Scan(&auditRequestID); err != nil {
		t.Fatal(err)
	}
	if auditRequestID == "" {
		t.Fatal("review did not persist HTTP request ID")
	}
}
