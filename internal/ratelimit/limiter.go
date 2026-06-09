package ratelimit

import (
	"sync"
	"time"
)

type Limiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	buckets  map[string]*bucket
	now      func() time.Time
}

type bucket struct {
	count       int
	windowStart time.Time
	lastSeen    time.Time
}

func New(limit int, window time.Duration) *Limiter {
	if limit <= 0 || window <= 0 {
		return nil
	}
	return &Limiter{limit: limit, window: window, buckets: make(map[string]*bucket), now: time.Now}
}

func (l *Limiter) Allow(key string) (bool, time.Duration) {
	if l == nil {
		return true, 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	b, ok := l.buckets[key]
	if !ok || now.Sub(b.windowStart) >= l.window {
		l.buckets[key] = &bucket{count: 1, windowStart: now, lastSeen: now}
		return true, 0
	}

	b.lastSeen = now
	if b.count >= l.limit {
		return false, l.window - now.Sub(b.windowStart)
	}
	b.count++
	return true, 0
}

func (l *Limiter) Cleanup(maxIdle time.Duration) int {
	if l == nil || maxIdle <= 0 {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	removed := 0
	for key, item := range l.buckets {
		if now.Sub(item.lastSeen) > maxIdle {
			delete(l.buckets, key)
			removed++
		}
	}
	return removed
}

func (l *Limiter) Size() int {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}
