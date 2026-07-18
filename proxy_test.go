package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestReverseProxyForwardsToUpstream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/some/path" {
			t.Errorf("expected path /some/path, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("upstream response"))
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("failed to parse upstream url: %v", err)
	}

	proxy := newReverseProxy(upstreamURL)

	req := httptest.NewRequest(http.MethodGet, "/some/path", nil)
	rec := httptest.NewRecorder()

	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != "upstream response" {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}
