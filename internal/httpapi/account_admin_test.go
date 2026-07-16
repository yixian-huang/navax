package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	adminpkg "github.com/yixian-huang/navax/internal/admin"
	"github.com/yixian-huang/navax/internal/auth"
	"github.com/yixian-huang/navax/internal/database"
)

func TestAccountHandlerLifecycle(t *testing.T) {
	db, authService, adminService, session, token := setupHandlerServices(t)
	_ = db
	router := handlerRouter(authService, adminService)

	response := performRequest(router, http.MethodGet, "/me/profile", nil, "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated profile status = %d", response.Code)
	}

	response = performRequest(router, http.MethodGet, "/me/profile", nil, token)
	if response.Code != http.StatusOK {
		t.Fatalf("profile status = %d: %s", response.Code, response.Body.String())
	}
	profile := decodeEnvelope(t, response)
	data := profile["data"].(map[string]any)
	if data["username"] != "owner" {
		t.Fatalf("profile data = %+v", data)
	}
	if _, leaked := data["passwordHash"]; leaked {
		t.Fatal("profile response leaked passwordHash")
	}

	response = performRequest(router, http.MethodPatch, "/me/profile", map[string]any{
		"username": "new_owner", "bio": "我的导航",
	}, token)
	if response.Code != http.StatusOK {
		t.Fatalf("update profile status = %d: %s", response.Code, response.Body.String())
	}

	second, secondToken, err := authService.Login(context.Background(), "owner@example.com", "initial-password", "second-browser")
	if err != nil {
		t.Fatal(err)
	}
	response = performRequest(router, http.MethodGet, "/me/sessions", nil, token)
	if response.Code != http.StatusOK {
		t.Fatalf("sessions status = %d: %s", response.Code, response.Body.String())
	}
	sessions := decodeEnvelope(t, response)["data"].([]any)
	if len(sessions) != 2 {
		t.Fatalf("session count = %d", len(sessions))
	}

	response = performRequest(router, http.MethodPatch, "/me/password", map[string]any{
		"currentPassword": "wrong", "newPassword": "replacement-password",
	}, token)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password status = %d", response.Code)
	}
	response = performRequest(router, http.MethodPatch, "/me/password", map[string]any{
		"currentPassword": "initial-password", "newPassword": "replacement-password",
	}, token)
	if response.Code != http.StatusOK {
		t.Fatalf("change password status = %d: %s", response.Code, response.Body.String())
	}
	if _, err := authService.Authenticate(context.Background(), secondToken); err == nil {
		t.Fatal("password change did not revoke the other session")
	}
	if second.ID == session.ID {
		t.Fatal("test did not create a distinct session")
	}

	response = performRequest(router, http.MethodDelete, "/me/sessions/missing-session", nil, token)
	if response.Code != http.StatusNotFound {
		t.Fatalf("missing session status = %d", response.Code)
	}
}

func TestAdminHandlerLifecycleAndAuthorization(t *testing.T) {
	_, authService, adminService, adminSession, adminToken := setupHandlerServices(t)
	router := handlerRouter(authService, adminService)
	actor := adminpkg.Actor{ID: adminSession.User.ID, Username: adminSession.User.Username, Role: "admin", Status: "active"}
	created, err := adminService.CreateInvitation(context.Background(), actor, adminpkg.InvitationCreate{
		Email: "user@example.com", MaxUses: 1, ExpiresInDays: 7, PublicBaseURL: "https://nav.ax",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, userToken, err := authService.Register(context.Background(), created.Token, auth.RegisterInput{
		Username: "regular", Email: "user@example.com", Password: "regular-password", Device: "test",
	})
	if err != nil {
		t.Fatal(err)
	}

	response := performRequest(router, http.MethodGet, "/admin/overview", nil, "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated admin status = %d", response.Code)
	}
	response = performRequest(router, http.MethodGet, "/admin/overview", nil, userToken)
	if response.Code != http.StatusForbidden {
		t.Fatalf("non-admin status = %d", response.Code)
	}
	response = performRequest(router, http.MethodGet, "/admin/overview", nil, adminToken)
	if response.Code != http.StatusOK {
		t.Fatalf("overview status = %d: %s", response.Code, response.Body.String())
	}
	health := decodeEnvelope(t, response)["data"].(map[string]any)["health"].(map[string]any)
	if health["version"] != "test-version" || health["goVersion"] == "" {
		t.Fatalf("health = %+v", health)
	}

	response = performRequest(router, http.MethodGet, "/admin/users?page=bad", nil, adminToken)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid pagination status = %d", response.Code)
	}
	response = performRequest(router, http.MethodGet, "/admin/users?page=1&pageSize=1", nil, adminToken)
	if response.Code != http.StatusOK {
		t.Fatalf("users status = %d: %s", response.Code, response.Body.String())
	}
	usersEnvelope := decodeEnvelope(t, response)
	if len(usersEnvelope["data"].([]any)) != 1 || usersEnvelope["meta"].(map[string]any)["total"].(float64) != 2 {
		t.Fatalf("users envelope = %+v", usersEnvelope)
	}

	response = performRequest(router, http.MethodPost, "/admin/invitations", map[string]any{
		"email": "guest@example.com", "maxUses": 1, "expiresInDays": 14, "sendEmail": false,
	}, adminToken)
	if response.Code != http.StatusCreated {
		t.Fatalf("create invitation status = %d: %s", response.Code, response.Body.String())
	}
	invitation := decodeEnvelope(t, response)["data"].(map[string]any)
	if invitation["token"] == "" || invitation["inviteUrl"] == "" {
		t.Fatalf("created invitation = %+v", invitation)
	}
	response = performRequest(router, http.MethodGet, "/admin/invitations", nil, adminToken)
	if response.Code != http.StatusOK {
		t.Fatalf("invitations status = %d: %s", response.Code, response.Body.String())
	}
	listed := decodeEnvelope(t, response)["data"].([]any)
	for _, item := range listed {
		if _, present := item.(map[string]any)["token"]; present {
			t.Fatal("invitation list leaked the complete token")
		}
	}

	response = performRequest(router, http.MethodPatch, "/admin/themes/kyoto", map[string]any{"default": true}, adminToken)
	if response.Code != http.StatusOK {
		t.Fatalf("set default theme status = %d: %s", response.Code, response.Body.String())
	}
	response = performRequest(router, http.MethodPatch, "/admin/themes/kyoto", map[string]any{"enabled": false}, adminToken)
	if response.Code != http.StatusConflict {
		t.Fatalf("disable default theme status = %d", response.Code)
	}

	response = performRequest(router, http.MethodPatch, "/admin/settings", map[string]any{
		"instanceName": "私有导航", "domain": map[string]any{"rootDomain": nil, "subdomainsEnabled": false},
	}, adminToken)
	if response.Code != http.StatusOK {
		t.Fatalf("update settings status = %d: %s", response.Code, response.Body.String())
	}
	settings := decodeEnvelope(t, response)["data"].(map[string]any)
	if settings["instanceName"] != "私有导航" || settings["domain"].(map[string]any)["rootDomain"] != nil {
		t.Fatalf("settings = %+v", settings)
	}

	response = performRequest(router, http.MethodGet, "/admin/audit?pageSize=100", nil, adminToken)
	if response.Code != http.StatusOK {
		t.Fatalf("audit status = %d: %s", response.Code, response.Body.String())
	}
	auditEnvelope := decodeEnvelope(t, response)
	if len(auditEnvelope["data"].([]any)) < 3 || auditEnvelope["meta"].(map[string]any)["requestId"] == "" {
		t.Fatalf("audit envelope = %+v", auditEnvelope)
	}
}

func setupHandlerServices(t *testing.T) (*sql.DB, *auth.Service, *adminpkg.Service, auth.Session, string) {
	t.Helper()
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	authService := auth.NewService(auth.NewSQLStore(db), "01234567890123456789012345678901", 24*time.Hour)
	session, token, err := authService.Bootstrap(ctx, "01234567890123456789012345678901", auth.BootstrapInput{
		Username: "owner", Email: "owner@example.com", Password: "initial-password",
		InstanceName: "nav.ax", PublicBaseURL: "https://nav.ax",
	})
	if err != nil {
		t.Fatal(err)
	}
	return db, authService, adminpkg.NewService(adminpkg.NewSQLStore(db)), session, token
}

func handlerRouter(authService *auth.Service, adminService *adminpkg.Service) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	NewAccountHandler(authService).Mount(router)
	NewAdminHandler(authService, adminService, AdminHandlerOptions{Version: "test-version", StartedAt: time.Now().Add(-time.Minute)}).Mount(router)
	return router
}

func performRequest(handler http.Handler, method, target string, body any, token string) *httptest.ResponseRecorder {
	var encoded *bytes.Reader
	if body == nil {
		encoded = bytes.NewReader(nil)
	} else {
		payload, _ := json.Marshal(body)
		encoded = bytes.NewReader(payload)
	}
	request := httptest.NewRequest(method, target, encoded)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func decodeEnvelope(t *testing.T, response *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
		t.Fatalf("decode response %q: %v", response.Body.String(), err)
	}
	return value
}
