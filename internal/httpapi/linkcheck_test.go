package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/yixian-huang/navax/internal/linkcheck"
)

func TestLinkCheckHandlerValidatesOpenAPIRequest(t *testing.T) {
	router := chi.NewRouter()
	NewLinkCheckHandler(linkcheck.NewService(nil)).MountProtected(router)
	request := httptest.NewRequest(http.MethodPost, "/pages/page_test/link-checks", strings.NewReader(`{"siteIds":[]}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"code":"VALIDATION_FAILED"`) {
		t.Fatalf("unexpected body: %s", response.Body.String())
	}
}
