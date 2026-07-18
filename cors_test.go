package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithCORSHandlesPreflight(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	handler := withCORS(inner, nil)

	req := httptest.NewRequest(http.MethodOptions, "/auth/otp/send", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://example.com" {
		t.Fatalf("expected origin to be reflected, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
	if called {
		t.Fatal("preflight request should not reach the wrapped handler")
	}
}

func TestWithCORSAddsHeadersToRealRequests(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := withCORS(inner, nil)

	req := httptest.NewRequest(http.MethodPost, "/auth/otp/send", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://example.com" {
		t.Fatal("expected CORS header on a real request too, not just preflight")
	}
}

func TestWithCORSRejectsDisallowedOrigin(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := withCORS(inner, []string{"http://allowed.example.com"})

	req := httptest.NewRequest(http.MethodPost, "/auth/otp/send", nil)
	req.Header.Set("Origin", "http://not-allowed.example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("expected no CORS header for a disallowed origin")
	}
}
