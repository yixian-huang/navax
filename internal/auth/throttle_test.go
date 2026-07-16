package auth

import (
	"testing"
	"time"
)

func TestLoginThrottleLocksAfterThreshold(t *testing.T) {
	throttle := newLoginThrottle()
	now := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	key := "user@example.com"

	for i := 0; i < throttle.threshold-1; i++ {
		throttle.fail(key, now)
		if _, blocked := throttle.retryAfter(key, now); blocked {
			t.Fatalf("locked after %d failures, want lock only at %d", i+1, throttle.threshold)
		}
	}
	throttle.fail(key, now)
	retryAfter, blocked := throttle.retryAfter(key, now)
	if !blocked || retryAfter != throttle.baseLock {
		t.Fatalf("retryAfter(threshold) = %v, %v; want %v, true", retryAfter, blocked, throttle.baseLock)
	}
}

func TestLoginThrottleBlockedAttemptDoesNotExtendLock(t *testing.T) {
	throttle := newLoginThrottle()
	now := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	key := "user@example.com"
	for i := 0; i < throttle.threshold; i++ {
		throttle.fail(key, now)
	}
	// retryAfter is read-only: repeated checks must not push the lock further out.
	first, _ := throttle.retryAfter(key, now)
	second, _ := throttle.retryAfter(key, now)
	if first != second {
		t.Fatalf("retryAfter drifted %v -> %v on read", first, second)
	}
	// After the window elapses, attempts are permitted again.
	if _, blocked := throttle.retryAfter(key, now.Add(throttle.baseLock)); blocked {
		t.Fatal("still blocked after lock window elapsed")
	}
}

func TestLoginThrottleEscalatesAndCaps(t *testing.T) {
	throttle := newLoginThrottle()
	now := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	key := "user@example.com"
	var last time.Duration
	for i := 0; i < throttle.threshold+10; i++ {
		throttle.fail(key, now)
		if d, blocked := throttle.retryAfter(key, now); blocked {
			last = d
		}
	}
	if last != throttle.maxLock {
		t.Fatalf("escalated lock = %v, want cap %v", last, throttle.maxLock)
	}
}

func TestLoginThrottleSuccessClears(t *testing.T) {
	throttle := newLoginThrottle()
	now := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	key := "user@example.com"
	for i := 0; i < throttle.threshold; i++ {
		throttle.fail(key, now)
	}
	throttle.success(key)
	if _, blocked := throttle.retryAfter(key, now); blocked {
		t.Fatal("success did not clear the lock")
	}
}

func TestLoginThrottleEvictsIdleEntries(t *testing.T) {
	throttle := newLoginThrottle()
	now := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	throttle.fail("stale@example.com", now)
	// A later failure on another key past the idle TTL should evict the stale one.
	throttle.fail("fresh@example.com", now.Add(throttle.idleTTL+time.Minute))
	throttle.mu.Lock()
	_, present := throttle.records["stale@example.com"]
	throttle.mu.Unlock()
	if present {
		t.Fatal("idle entry was not evicted")
	}
}
