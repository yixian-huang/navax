package auth

import (
	"sync"
	"time"
)

// loginThrottle applies per-account backoff on repeated failed logins, keyed by
// normalized email. It complements the per-IP rate limiter in the HTTP layer:
// the IP limiter bounds a single source, while this bounds attempts against a
// single account even when the attacker rotates source addresses.
//
// The lock is temporary and self-healing — it never disables the account, so a
// forgotten-password user only waits out a short window (and can use account
// recovery), and an attacker cannot permanently lock a victim out. State is
// in-process, matching the app's single-process design; a restart clears it,
// which is acceptable for a defense-in-depth throttle.
type loginThrottle struct {
	mu        sync.Mutex
	records   map[string]throttleState
	maxKeys   int
	threshold int
	baseLock  time.Duration
	maxLock   time.Duration
	idleTTL   time.Duration
}

type throttleState struct {
	failures  int
	lockUntil time.Time
	updatedAt time.Time
}

func newLoginThrottle() *loginThrottle {
	return &loginThrottle{
		records:   make(map[string]throttleState),
		maxKeys:   50_000,
		threshold: 5,
		baseLock:  time.Minute,
		maxLock:   15 * time.Minute,
		idleTTL:   30 * time.Minute,
	}
}

// retryAfter reports the remaining lock duration for key, or false if attempts
// are currently permitted. It never mutates counters, so rejected attempts do
// not extend the lock.
func (t *loginThrottle) retryAfter(key string, now time.Time) (time.Duration, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	state, ok := t.records[key]
	if !ok {
		return 0, false
	}
	if state.lockUntil.After(now) {
		return state.lockUntil.Sub(now), true
	}
	return 0, false
}

// fail records one failed attempt for key and, once the consecutive-failure
// threshold is reached, arms an exponentially growing lock capped at maxLock.
func (t *loginThrottle) fail(key string, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.evict(now)
	state, tracked := t.records[key]
	if !tracked && len(t.records) >= t.maxKeys {
		return // at capacity with live locks; degrade to allow rather than evict a live lock
	}
	state.failures++
	state.updatedAt = now
	if state.failures >= t.threshold {
		lock := t.baseLock << uint(min(state.failures-t.threshold, 4))
		if lock > t.maxLock {
			lock = t.maxLock
		}
		state.lockUntil = now.Add(lock)
	}
	t.records[key] = state
}

// success clears any recorded failures for key after a valid authentication.
func (t *loginThrottle) success(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.records, key)
}

// evict drops entries that are past their lock and idle beyond idleTTL. Called
// under the lock from fail, which is the only path that grows the map.
func (t *loginThrottle) evict(now time.Time) {
	for key, state := range t.records {
		if state.lockUntil.After(now) {
			continue
		}
		if now.Sub(state.updatedAt) >= t.idleTTL {
			delete(t.records, key)
		}
	}
}
