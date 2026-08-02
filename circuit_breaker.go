package main

import (
	"sync"
	"time"
)

type circuitState int

const (
	circuitClosed circuitState = iota
	circuitOpen
	circuitHalfOpen
)

func (s circuitState) String() string {
	switch s {
	case circuitOpen:
		return "open"
	case circuitHalfOpen:
		return "half-open"
	default:
		return "closed"
	}
}

// CircuitBreaker is one per upstream (keyed by the route prefix in router.go), not global --
// one upstream having a bad day shouldn't trip the breaker for every other service behind the
// same gateway.
type CircuitBreaker struct {
	mu               sync.Mutex
	state            circuitState
	consecutiveFails int
	openedAt         time.Time

	failureThreshold int           // consecutive failures before opening
	openDuration     time.Duration // how long to stay open before trying a half-open probe
}

func NewCircuitBreaker(failureThreshold int, openDuration time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:            circuitClosed,
		failureThreshold: failureThreshold,
		openDuration:     openDuration,
	}
}

// Allow reports whether a request should be let through. In the open state, a single probe
// request is allowed through once openDuration has elapsed (moving to half-open) so the
// breaker can find out if the upstream recovered without waiting for a full close.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case circuitOpen:
		if time.Since(cb.openedAt) >= cb.openDuration {
			cb.state = circuitHalfOpen
			return true
		}
		return false
	default:
		return true
	}
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.consecutiveFails = 0
	cb.state = circuitClosed
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// a failed half-open probe reopens immediately, doesn't need to re-accumulate the full
	// failure threshold
	if cb.state == circuitHalfOpen {
		cb.state = circuitOpen
		cb.openedAt = time.Now()
		return
	}

	cb.consecutiveFails++
	if cb.consecutiveFails >= cb.failureThreshold {
		cb.state = circuitOpen
		cb.openedAt = time.Now()
	}
}

func (cb *CircuitBreaker) State() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state.String()
}

// CircuitBreakerRegistry hands out one breaker per upstream key, created lazily.
type CircuitBreakerRegistry struct {
	mu       sync.Mutex
	breakers map[string]*CircuitBreaker
}

func NewCircuitBreakerRegistry() *CircuitBreakerRegistry {
	return &CircuitBreakerRegistry{breakers: make(map[string]*CircuitBreaker)}
}

func (r *CircuitBreakerRegistry) For(upstreamKey string) *CircuitBreaker {
	r.mu.Lock()
	defer r.mu.Unlock()
	cb, ok := r.breakers[upstreamKey]
	if !ok {
		// 5 consecutive failures trips it, stays open 10s before probing again -- not
		// configurable per-upstream yet, one policy for every route
		cb = NewCircuitBreaker(5, 10*time.Second)
		r.breakers[upstreamKey] = cb
	}
	return cb
}

func (r *CircuitBreakerRegistry) Snapshot() map[string]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]string, len(r.breakers))
	for k, cb := range r.breakers {
		out[k] = cb.State()
	}
	return out
}
