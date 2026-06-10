package delivery

import (
	"net/http"
	"testing"
	"time"
)

func TestBackoff(t *testing.T) {
	cases := []struct {
		attempts int
		want     time.Duration
	}{
		{attempts: 0, want: 30 * time.Second},
		{attempts: 1, want: time.Minute},
		{attempts: 2, want: 2 * time.Minute},
		{attempts: 10, want: 15 * time.Minute},
	}

	for _, item := range cases {
		if got := backoff(item.attempts); got != item.want {
			t.Fatalf("backoff(%d) = %s, want %s", item.attempts, got, item.want)
		}
	}
}

func TestRetryAfter(t *testing.T) {
	resp := &http.Response{Header: make(http.Header)}
	resp.Header.Set("Retry-After", "42")
	if got := retryAfter(resp, time.Minute); got != 42*time.Second {
		t.Fatalf("retryAfter() = %s, want 42s", got)
	}
}

func TestRetryAfterFallback(t *testing.T) {
	resp := &http.Response{Header: make(http.Header)}
	resp.Header.Set("Retry-After", "invalid")
	if got := retryAfter(resp, time.Minute); got != time.Minute {
		t.Fatalf("retryAfter() = %s, want 1m", got)
	}
}
