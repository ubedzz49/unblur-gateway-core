package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithCorrelationIDGeneratesWhenAbsent(t *testing.T) {
	var seen string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = requestIDFromContext(r.Context())
	})
	handler := withCorrelationID(inner)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if seen == "" {
		t.Fatal("expected a generated request id in context")
	}
	if rec.Header().Get(requestIDHeader) != seen {
		t.Fatalf("expected response header %q to match context id %q, got %q", requestIDHeader, seen, rec.Header().Get(requestIDHeader))
	}
}

func TestWithCorrelationIDReusesIncomingHeader(t *testing.T) {
	const incoming = "existing-request-id-123"
	var seen string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = requestIDFromContext(r.Context())
		if r.Header.Get(requestIDHeader) != incoming {
			t.Fatalf("expected incoming header to be forwarded, got %q", r.Header.Get(requestIDHeader))
		}
	})
	handler := withCorrelationID(inner)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set(requestIDHeader, incoming)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if seen != incoming {
		t.Fatalf("expected reused id %q, got %q", incoming, seen)
	}
	if rec.Header().Get(requestIDHeader) != incoming {
		t.Fatalf("expected response header to reuse incoming id, got %q", rec.Header().Get(requestIDHeader))
	}
}

func TestWithCorrelationIDGeneratesUniqueIDs(t *testing.T) {
	seenIDs := map[string]bool{}
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenIDs[requestIDFromContext(r.Context())] = true
	})
	handler := withCorrelationID(inner)

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	if len(seenIDs) != 5 {
		t.Fatalf("expected 5 unique request ids, got %d", len(seenIDs))
	}
}
