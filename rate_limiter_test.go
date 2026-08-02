package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRateLimiterAllowsUpToBurst(t *testing.T) {
	rl := NewRateLimiter(3, 1)
	for i := 0; i < 3; i++ {
		if !rl.Allow("client-a") {
			t.Fatalf("request %d should have been allowed within burst", i)
		}
	}
	if rl.Allow("client-a") {
		t.Error("4th request should be rejected once burst is exhausted")
	}
}

func TestRateLimiterTracksClientsIndependently(t *testing.T) {
	rl := NewRateLimiter(1, 1)
	if !rl.Allow("client-a") {
		t.Fatal("client-a's first request should be allowed")
	}
	if !rl.Allow("client-b") {
		t.Error("client-b should have its own independent budget")
	}
}

func TestClientKeyPrefersUserID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/seminars", nil)
	req.Header.Set("X-User-Id", "user-123")
	req.RemoteAddr = "10.0.0.1:1234"

	if got := clientKey(req); got != "user:user-123" {
		t.Errorf("expected user:user-123, got %q", got)
	}
}

func TestClientKeyFallsBackToRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/auth/otp/send", nil)
	req.RemoteAddr = "10.0.0.1:1234"

	if got := clientKey(req); got != "ip:10.0.0.1:1234" {
		t.Errorf("expected ip:10.0.0.1:1234, got %q", got)
	}
}

func TestWithRateLimitRejects429OnceExhausted(t *testing.T) {
	rl := NewRateLimiter(1, 1)
	called := 0
	handler := withRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusOK)
	}), rl)

	req := httptest.NewRequest(http.MethodGet, "/seminars", nil)
	req.Header.Set("X-User-Id", "user-429")

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, req)
	if first.Code != http.StatusOK {
		t.Fatalf("first request should succeed, got %d", first.Code)
	}

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, req)
	if second.Code != http.StatusTooManyRequests {
		t.Errorf("second request should be rate limited, got %d", second.Code)
	}
	if called != 1 {
		t.Errorf("handler should only have been called once, got %d", called)
	}
}
