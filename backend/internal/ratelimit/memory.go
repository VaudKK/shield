package ratelimit

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// InMemoryLimiter is a per-key token-bucket limiter. It keeps state in
// process memory, so it only rate-limits within a single instance; the
// application still runs correctly without Redis in local development.
// In a multi-instance production deployment, swap in a Redis-backed
// implementation of Limiter for limits shared across instances.
type InMemoryLimiter struct {
	rps   rate.Limit
	burst int

	mu       sync.Mutex
	buckets  map[string]*bucket
	stopOnce sync.Once
	stopCh   chan struct{}
}

type bucket struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewInMemoryLimiter allows `burst` requests immediately, refilling at
// `requestsPerInterval` per `interval`.
func NewInMemoryLimiter(requestsPerInterval int, interval time.Duration, burst int) *InMemoryLimiter {
	l := &InMemoryLimiter{
		rps:     rate.Limit(float64(requestsPerInterval) / interval.Seconds()),
		burst:   burst,
		buckets: make(map[string]*bucket),
		stopCh:  make(chan struct{}),
	}
	go l.evictStaleBuckets()
	return l
}

func (l *InMemoryLimiter) Allow(_ context.Context, key string) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{limiter: rate.NewLimiter(l.rps, l.burst)}
		l.buckets[key] = b
	}
	b.lastSeen = time.Now()

	return b.limiter.Allow(), nil
}

func (l *InMemoryLimiter) Close() {
	l.stopOnce.Do(func() { close(l.stopCh) })
}

func (l *InMemoryLimiter) evictStaleBuckets() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-l.stopCh:
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-30 * time.Minute)
			l.mu.Lock()
			for key, b := range l.buckets {
				if b.lastSeen.Before(cutoff) {
					delete(l.buckets, key)
				}
			}
			l.mu.Unlock()
		}
	}
}
