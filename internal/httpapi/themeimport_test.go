package httpapi_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/yixian-huang/navax/internal/catalog"
	"github.com/yixian-huang/navax/internal/database"
	"github.com/yixian-huang/navax/internal/httpapi"
	"github.com/yixian-huang/navax/internal/themeimport"
	"github.com/yixian-huang/navax/internal/themes"
)

// newThemeImportTestServer 组一个最小依赖的 ThemeImportHandler：/themes/validate
// 不读 session，因此无需像生产那样套 RequireSession 中间件。
func newThemeImportTestServer(t *testing.T) (*sql.DB, http.Handler) {
	t.Helper()
	db, err := database.OpenAndMigrate(context.Background(), database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatalf("OpenAndMigrate() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	service := themeimport.NewService(themes.NewStore(db), themeimport.NewGitHubClient(nil, nil, nil, ""), 10)
	handler := httpapi.NewThemeImportHandler(service, catalog.NewService(db))
	router := chi.NewRouter()
	router.Route("/api/v1", func(api chi.Router) {
		handler.MountProtected(api)
	})
	return db, router
}

func TestThemeValidateRejectsNonMultipartContentType(t *testing.T) {
	_, router := newThemeImportTestServer(t)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/themes/validate", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415: %s", response.Code, response.Body.String())
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Code != "UNSUPPORTED_MEDIA_TYPE" {
		t.Fatalf("code = %q, want UNSUPPORTED_MEDIA_TYPE", body.Code)
	}
}

// TestThemeValidateRejectsMalformedMultipartBody 确认 Content-Type 正确但
// multipart 主体本身损坏时得到 422 VALIDATION_FAILED，而不是被误判成
// 413 PAYLOAD_TOO_LARGE（旧实现把 ParseMultipartForm 的所有错误都标成体积超限）。
func TestThemeValidateRejectsMalformedMultipartBody(t *testing.T) {
	_, router := newThemeImportTestServer(t)

	// 合法的 Content-Type，但主体不是该 boundary 下的合法 multipart 编码。
	request := httptest.NewRequest(http.MethodPost, "/api/v1/themes/validate", strings.NewReader("not-a-multipart-body"))
	request.Header.Set("Content-Type", "multipart/form-data; boundary=xyz")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422: %s", response.Code, response.Body.String())
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Code != "VALIDATION_FAILED" {
		t.Fatalf("code = %q, want VALIDATION_FAILED", body.Code)
	}
}
