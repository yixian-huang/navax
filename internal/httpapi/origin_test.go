package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVerifyOriginForAuthenticatedWrite(t *testing.T) {
	handler := VerifyOrigin("https://nav.ax")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodPost, "https://nav.ax/api/v1/auth/logout", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "token"})
	request.Header.Set("Origin", "https://evil.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-origin status = %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "https://nav.ax/api/v1/auth/logout", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "token"})
	request.Header.Set("Origin", "https://nav.ax")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("same-origin status = %d", response.Code)
	}
}

func TestVerifyOriginAllowsPublicLoginWithoutCookie(t *testing.T) {
	handler := VerifyOrigin("https://nav.ax")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodPost, "https://nav.ax/api/v1/auth/login", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("public login status = %d", response.Code)
	}
}
