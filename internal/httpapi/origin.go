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
	if !strings.EqualFold(left.Scheme, right.Scheme) {
		return false
	}
	// Treat localhost and 127.0.0.1 as equivalent so local Vite (either form)
	// can talk to a backend that was started with the other host string.
	return normalizeLocalHost(left.Host) == normalizeLocalHost(right.Host)
}

func normalizeLocalHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	// host may be "localhost:3000" or "127.0.0.1:3000"
	if h, p, err := splitHostPortLoose(host); err == nil {
		if h == "127.0.0.1" {
			h = "localhost"
		}
		if p == "" {
			return h
		}
		return h + ":" + p
	}
	if host == "127.0.0.1" {
		return "localhost"
	}
	return host
}

func splitHostPortLoose(host string) (string, string, error) {
	// net.SplitHostPort requires brackets for IPv6; we only care about local dev hosts.
	if i := strings.LastIndex(host, ":"); i > 0 && !strings.Contains(host, "]") {
		return host[:i], host[i+1:], nil
	}
	return host, "", nil
}
