package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAbuseProtectionLimitsLoginByPeer(t *testing.T) {
	handler := AbuseProtection()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	for attempt := 1; attempt <= 11; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		request.RemoteAddr = "203.0.113.20:12345"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if attempt <= 10 && response.Code != http.StatusNoContent {
			t.Fatalf("attempt %d status = %d", attempt, response.Code)
		}
		if attempt == 11 && (response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") == "") {
			t.Fatalf("limited response = %d, headers=%v", response.Code, response.Header())
		}
	}
}
