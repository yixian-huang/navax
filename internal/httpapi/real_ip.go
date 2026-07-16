package httpapi

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// RealIP rewrites r.RemoteAddr to the real client IP when the immediate peer is
// a configured trusted proxy, so downstream consumers (rate limiting, analytics)
// key off the client rather than the proxy. With no trusted proxies configured it
// is a no-op, and X-Forwarded-For from an untrusted peer is never trusted — this
// prevents header spoofing while making per-IP controls work behind a TLS proxy.
func RealIP(trusted []netip.Prefix) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if len(trusted) == 0 {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if client := resolveClientIP(r, trusted); client != "" {
				r.RemoteAddr = net.JoinHostPort(client, "0")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// resolveClientIP returns the rightmost X-Forwarded-For entry that is not itself a
// trusted proxy, but only when the direct peer is trusted. Returns "" to keep RemoteAddr.
func resolveClientIP(r *http.Request, trusted []netip.Prefix) string {
	peer, err := netip.ParseAddr(peerAddress(r.RemoteAddr))
	if err != nil || !ipTrusted(peer, trusted) {
		return ""
	}
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded == "" {
		return ""
	}
	parts := strings.Split(forwarded, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		candidate, err := netip.ParseAddr(strings.TrimSpace(parts[i]))
		if err != nil {
			continue
		}
		candidate = candidate.Unmap()
		if ipTrusted(candidate, trusted) {
			continue
		}
		return candidate.String()
	}
	return ""
}

func ipTrusted(addr netip.Addr, trusted []netip.Prefix) bool {
	addr = addr.Unmap()
	for _, prefix := range trusted {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}
