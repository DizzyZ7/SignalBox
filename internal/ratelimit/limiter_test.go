package ratelimit

import (
	"testing"
	"time"
)

func TestLimiterAllowsWithinLimit(t *testing.T) {
	limiter := New(2, time.Minute)
	if ok, _ := limiter.Allow("client"); !ok {
		t.Fatal("first request should be allowed")
	}
	if ok, _ := limiter.Allow("client"); !ok {
		t.Fatal("second request should be allowed")
	}
	if ok, _ := limiter.Allow("client"); ok {
		t.Fatal("third request should be rejected")
	}
}

func TestLimiterResetsAfterWindow(t *testing.T) {
	limiter := New(1, time.Minute)
	current := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return current }

	if ok, _ := limiter.Allow("client"); !ok {
		t.Fatal("first request should be allowed")
	}
	if ok, _ := limiter.Allow("client"); ok {
		t.Fatal("second request should be rejected")
	}

	current = current.Add(time.Minute)
	if ok, _ := limiter.Allow("client"); !ok {
		t.Fatal("request after window should be allowed")
	}
}

func TestDisabledLimiterAllowsAll(t *testing.T) {
	limiter := New(0, time.Minute)
	if ok, _ := limiter.Allow("client"); !ok {
		t.Fatal("disabled limiter should allow request")
	}
}
