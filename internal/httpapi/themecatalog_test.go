package httpapi

import (
	"archive/zip"
	"bytes"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/yixian-huang/navax/internal/security"
	"github.com/yixian-huang/navax/internal/themecatalog"
	"github.com/yixian-huang/navax/internal/themeimport"
	"github.com/yixian-huang/navax/internal/themes"
)

// auroraManifest/auroraZip build a minimal legal theme package the same way
// internal/themeimport/service_test.go's sampleManifest/sampleZip do (theme
// package Go structs use plain map[string]string tokens, not their own named
// types — going through ImportZip's real zip→manifest→compile pipeline
// avoids hand-constructing themes.Manifest/themes.Compiled by field name).
const auroraManifest = `{
  "specVersion": 1, "id": "aurora", "name": "Aurora", "version": "1.0.0",
  "author": "t", "license": "MIT", "mode": "light", "vibe": "serious",
  "swatches": ["#f5f3ff", "#8b5cf6", "#1e1b4b"], "tier": 1,
  "tokens": {
    "font": { "heading": "system-ui", "body": "system-ui", "label": "system-ui", "mono": "monospace" },
    "color": {
      "background": { "50": "0.985 0.010 300" },
      "foreground": { "900": "0.210 0.040 300" },
      "primary":    { "500": "0.585 0.200 300" },
      "accent":     { "500": "0.700 0.150 160" }
    }
  }
}`

func auroraZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, data := range map[string][]byte{
		"theme.json": []byte(auroraManifest),
		"theme.css":  []byte(`[data-nx="site-card"] { border-radius: var(--radius-md); }`),
	} {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestThemeCatalogHandlerLifecycle(t *testing.T) {
	db, authService, _, _, token := setupHandlerServices(t)
	themeStore := themes.NewStore(db)
	stamp := time.Now().UTC()
	passwordHash, err := security.HashPassword("integration-password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO users (id, username, email, password_hash, role, status, created_at, updated_at)
		VALUES ('usr_tcr_owner', 'requester', 'requester@example.com', ?, 'user', 'active', ?, ?)`,
		passwordHash, stamp.Format(time.RFC3339Nano), stamp.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	importService := themeimport.NewService(themeStore, nil, 10)
	installed, err := importService.ImportZip(t.Context(), "usr_tcr_owner", auroraZip(t))
	if err != nil {
		t.Fatal(err)
	}

	// requester 会话:用真实密码登录换 token,不走 bootstrap(那是 setupHandlerServices
	// 已经用过的管理员账号)。auth.Service.Login 按 username 或 email 匹配。
	_, requesterToken, err := authService.Login(t.Context(), "requester", "integration-password", "e2e-test")
	if err != nil {
		t.Fatal(err)
	}

	service := themecatalog.NewService(themecatalog.NewSQLStore(db))
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	handler := NewThemeCatalogHandler(authService, service)
	router.Group(func(protected chi.Router) {
		protected.Use(RequireSession(authService))
		handler.MountUserRoutes(protected)
		protected.Route("/admin", func(admin chi.Router) {
			admin.Use(RequireAdmin)
			handler.MountAdminRoutes(admin)
		})
	})

	response := performRequest(router, http.MethodPost, "/me/themes/"+installed.ThemeID+"/catalog-request", nil, requesterToken)
	if response.Code != http.StatusCreated {
		t.Fatalf("submit status = %d: %s", response.Code, response.Body.String())
	}
	created := decodeEnvelope(t, response)["data"].(map[string]any)
	requestID := created["id"].(string)
	if created["status"] != "pending" || created["slug"] != "aurora" {
		t.Fatalf("created request = %+v", created)
	}

	response = performRequest(router, http.MethodGet, "/admin/theme-catalog-requests?status=pending", nil, token)
	if response.Code != http.StatusOK {
		t.Fatalf("admin list status = %d: %s", response.Code, response.Body.String())
	}
	listed := decodeEnvelope(t, response)
	if listed["meta"].(map[string]any)["total"].(float64) != 1 {
		t.Fatalf("admin list = %+v", listed)
	}

	response = performRequest(router, http.MethodPatch, "/admin/theme-catalog-requests/"+requestID,
		map[string]any{"decision": "approve"}, token)
	if response.Code != http.StatusOK {
		t.Fatalf("approve status = %d: %s", response.Code, response.Body.String())
	}
	approved := decodeEnvelope(t, response)["data"].(map[string]any)
	if approved["status"] != "approved" {
		t.Fatalf("approved request = %+v", approved)
	}
	var scope string
	if err := db.QueryRow(`SELECT scope FROM themes WHERE id = ?`, installed.ThemeID).Scan(&scope); err != nil {
		t.Fatal(err)
	}
	if scope != "catalog" {
		t.Fatalf("scope = %q, want catalog", scope)
	}
}
