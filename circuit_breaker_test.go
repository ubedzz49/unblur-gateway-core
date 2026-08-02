package main

import (
	"testing"
	"time"
)

func TestCircuitBreakerOpensAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Minute)
	for i := 0; i < 3; i++ {
		if !cb.Allow() {
			t.Fatalf("request %d should be allowed while closed", i)
		}
		cb.RecordFailure()
	}
	if cb.State() != "open" {
		t.Errorf("expected open after 3 consecutive failures, got %q", cb.State())
	}
	if cb.Allow() {
		t.Error("should not allow requests while open")
	}
}

func TestCircuitBreakerRecoversOnSuccess(t *testing.T) {
	cb := NewCircuitBreaker(2, time.Minute)
	cb.RecordFailure()
	cb.RecordSuccess()
	cb.RecordFailure()
	if cb.State() != "closed" {
		t.Errorf("a success should reset the failure count, expected still closed, got %q", cb.State())
	}
}

func TestCircuitBreakerHalfOpenAfterCooldown(t *testing.T) {
	cb := NewCircuitBreaker(1, 10*time.Millisecond)
	cb.RecordFailure()
	if cb.State() != "open" {
		t.Fatalf("expected open, got %q", cb.State())
	}
	if cb.Allow() {
		t.Fatal("should not allow immediately after opening")
	}

	time.Sleep(20 * time.Millisecond)
	if !cb.Allow() {
		t.Fatal("should allow a probe request once the cooldown elapses")
	}
}

func TestCircuitBreakerFailedProbeReopensImmediately(t *testing.T) {
	cb := NewCircuitBreaker(1, 10*time.Millisecond)
	cb.RecordFailure()
	time.Sleep(20 * time.Millisecond)
	cb.Allow() // moves to half-open
	cb.RecordFailure()
	if cb.State() != "open" {
		t.Errorf("a failed probe should reopen the breaker, got %q", cb.State())
	}
}

func TestCircuitBreakerRegistryIsolatesUpstreams(t *testing.T) {
	registry := NewCircuitBreakerRegistry()
	a := registry.For("http://service-a")
	b := registry.For("http://service-b")

	for i := 0; i < 5; i++ {
		a.RecordFailure()
	}
	if a.State() != "open" {
		t.Fatal("service-a's breaker should be open")
	}
	if b.State() != "closed" {
		t.Error("service-b's breaker should be unaffected by service-a's failures")
	}
}
