package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestUpstream(t *testing.T, label string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(label))
	}))
}

func TestRouterMatchesLongestPrefix(t *testing.T) {
	users := newTestUpstream(t, "users")
	defer users.Close()
	auth := newTestUpstream(t, "auth")
	defer auth.Close()

	router, err := NewRouter([]RouteConfig{
		{Prefix: "/users", Upstream: users.URL},
		{Prefix: "/users/me/auth", Upstream: auth.URL},
	})
	if err != nil {
		t.Fatalf("failed to build router: %v", err)
	}

	tests := []struct {
		path string
		want string
	}{
		{"/users/123", "users"},
		{"/users/me/auth/token", "auth"},
	}

	for _, tc := range tests {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Body.String() != tc.want {
			t.Errorf("path %s: expected upstream %q, got %q", tc.path, tc.want, rec.Body.String())
		}
	}
}

func TestRouterReturns404WithNoMatchingRoute(t *testing.T) {
	router, err := NewRouter([]RouteConfig{{Prefix: "/users", Upstream: "http://example.invalid"}})
	if err != nil {
		t.Fatalf("failed to build router: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/doubts", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestParseRouteConfigsRejectsInvalidJson(t *testing.T) {
	if _, err := ParseRouteConfigs("not json"); err == nil {
		t.Fatal("expected an error for invalid json, got nil")
	}
}
