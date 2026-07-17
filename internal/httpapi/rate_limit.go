package httpapi

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type rateRule struct {
	method string
	match  func(string) bool
	limit  int
	window time.Duration
}

type rateEntry struct {
	count int
	reset time.Time
}

type abuseLimiter struct {
	mu      sync.Mutex
	entries map[string]rateEntry
	now     func() time.Time
	maxKeys int
	ops     uint64
}

func AbuseProtection() func(http.Handler) http.Handler {
	limiter := &abuseLimiter{entries: make(map[string]rateEntry), now: time.Now, maxKeys: 50_000}
	rules := []rateRule{
		{http.MethodPost, exactPath("/api/v1/auth/login"), 10, 5 * time.Minute},
		{http.MethodPost, exactPath("/api/v1/auth/password/forgot"), 5, 15 * time.Minute},
		{http.MethodPost, exactPath("/api/v1/auth/password/reset"), 10, 15 * time.Minute},
		{http.MethodPost, exactPath("/api/v1/bootstrap"), 5, 10 * time.Minute},
		{http.MethodPost, func(path string) bool {
			return strings.HasPrefix(path, "/api/v1/auth/invitations/") && strings.HasSuffix(path, "/register")
		}, 10, 10 * time.Minute},
		{http.MethodPatch, exactPath("/api/v1/me/password"), 10, 15 * time.Minute},
		{http.MethodPost, func(path string) bool {
			return strings.HasPrefix(path, "/api/v1/admin/backups/") && strings.HasSuffix(path, "/restore-token")
		}, 5, 15 * time.Minute},
		{http.MethodPost, exactPath("/api/v1/public/events"), 120, time.Minute},
		{http.MethodPost, func(path string) bool { return strings.HasSuffix(path, "/link-checks") }, 10, time.Minute},
		{http.MethodPost, exactPath("/api/v1/link-preview"), 60, time.Minute},
		{http.MethodPost, exactPath("/api/v1/assets"), 30, time.Minute},
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for index, rule := range rules {
				if r.Method != rule.method || !rule.match(r.URL.Path) {
					continue
				}
				key := strconv.Itoa(index) + ":" + peerAddress(r.RemoteAddr)
				allowed, retryAfter := limiter.allow(key, rule.limit, rule.window)
				if !allowed {
					w.Header().Set("Retry-After", strconv.Itoa(max(1, int(retryAfter.Seconds()))))
					WriteError(w, r, http.StatusTooManyRequests, "RATE_LIMITED", "请求过于频繁，请稍后重试", nil)
					return
				}
				break
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (l *abuseLimiter) allow(key string, limit int, window time.Duration) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.ops++
	if l.ops%1000 == 0 {
		for candidate, entry := range l.entries {
			if !entry.reset.After(now) {
				delete(l.entries, candidate)
			}
		}
	}
	entry, exists := l.entries[key]
	if !exists || !entry.reset.After(now) {
		if !exists && len(l.entries) >= l.maxKeys {
			// Prefer dropping expired keys; if still full, evict one arbitrary
			// entry so a table fill cannot lock out every new peer.
			for candidate, stale := range l.entries {
				if !stale.reset.After(now) {
					delete(l.entries, candidate)
				}
			}
			if len(l.entries) >= l.maxKeys {
				for candidate := range l.entries {
					delete(l.entries, candidate)
					break
				}
			}
		}
		l.entries[key] = rateEntry{count: 1, reset: now.Add(window)}
		return true, 0
	}
	if entry.count >= limit {
		return false, entry.reset.Sub(now)
	}
	entry.count++
	l.entries[key] = entry
	return true, 0
}

func exactPath(expected string) func(string) bool {
	return func(path string) bool { return path == expected }
}

func peerAddress(remote string) string {
	if host, _, err := net.SplitHostPort(strings.TrimSpace(remote)); err == nil {
		return host
	}
	return strings.TrimSpace(remote)
}
