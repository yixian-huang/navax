package httpapi

import (
	"net/http"
	"net/url"
	"strings"
)

// VerifyOrigin rejects cross-site state changes on non-safe methods when a
// browser Origin/Referer is present. Machine clients (curl, scripts) typically
// omit Origin; those requests are allowed. When Origin/Referer is sent it must
// match publicBaseURL, which blocks login CSRF and authenticated write CSRF.
func VerifyOrigin(publicBaseURL string) func(http.Handler) http.Handler {
	allowed, _ := url.Parse(publicBaseURL)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isSafeMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}
			candidate := strings.TrimSpace(r.Header.Get("Origin"))
			if candidate == "" {
				candidate = strings.TrimSpace(r.Header.Get("Referer"))
			}
			// No Origin/Referer: treat as non-browser / machine client.
			if candidate == "" {
				next.ServeHTTP(w, r)
				return
			}
			parsed, err := url.Parse(candidate)
			if err != nil || !sameOrigin(parsed, allowed) {
				WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "请求来源校验失败", nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

func sameOrigin(left, right *url.URL) bool {
	if left == nil || right == nil {
		return false
	}
	return strings.EqualFold(left.Scheme, right.Scheme) && strings.EqualFold(left.Host, right.Host)
}
