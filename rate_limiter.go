package main

import (
	"net/http"
	"sync"
	"time"
)

// simple in-memory token bucket per client key (X-User-Id if the request already has a
// gateway-verified identity, else remote IP for pre-auth requests like /auth/*). Good enough for
// a single-instance gateway; a multi-instance deployment would need this backed by Redis instead
// -- flagged in KNOWN_GAPS.md rather than built now, since gateway-core only ever runs as one
// task today.
type tokenBucket struct {
	tokens     float64
	lastRefill time.Time
}

type RateLimiter struct {
	mu         sync.Mutex
	buckets    map[string]*tokenBucket
	maxTokens  float64
	refillRate float64 // tokens per second
}

func NewRateLimiter(maxTokens float64, refillPerSecond float64) *RateLimiter {
	return &RateLimiter{
		buckets:    make(map[string]*tokenBucket),
		maxTokens:  maxTokens,
		refillRate: refillPerSecond,
	}
}

func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[key]
	if !ok {
		b = &tokenBucket{tokens: rl.maxTokens - 1, lastRefill: now}
		rl.buckets[key] = b
		return true
	}

	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens = min(rl.maxTokens, b.tokens+elapsed*rl.refillRate)
	b.lastRefill = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func clientKey(r *http.Request) string {
	if userID := r.Header.Get("X-User-Id"); userID != "" {
		return "user:" + userID
	}
	// r.RemoteAddr includes the port; good enough as a rough per-connection key for
	// pre-auth requests, not meant to survive a client behind a shared NAT gateway perfectly
	return "ip:" + r.RemoteAddr
}

// withRateLimit rejects a client once they exceed the configured request budget, 429 with a
// Retry-After hint. Applied after JWT verification so the X-User-Id key is available for
// authenticated requests, and after correlation ID assignment so a 429 still gets logged with one.
func withRateLimit(next http.Handler, limiter *RateLimiter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow(clientKey(r)) {
			recordRateLimited()
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limit exceeded, try again shortly"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}
