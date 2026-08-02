package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRoutesHandlerGetReturnsCurrentRoutes(t *testing.T) {
	router, err := NewRouter([]RouteConfig{{Prefix: "/users", Upstream: "http://user-service"}})
	if err != nil {
		t.Fatalf("failed to build router: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/internal/routes", nil)
	rec := httptest.NewRecorder()
	routesHandler(router)(rec, req)

	var configs []RouteConfig
	if err := json.NewDecoder(rec.Body).Decode(&configs); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(configs) != 1 || configs[0].Prefix != "/users" {
		t.Errorf("unexpected routes: %+v", configs)
	}
}

func TestRoutesHandlerPostReplacesTable(t *testing.T) {
	router, err := NewRouter([]RouteConfig{{Prefix: "/users", Upstream: "http://user-service"}})
	if err != nil {
		t.Fatalf("failed to build router: %v", err)
	}

	payload, _ := json.Marshal([]RouteConfig{{Prefix: "/doubts", Upstream: "http://doubt-service"}})
	req := httptest.NewRequest(http.MethodPost, "/internal/routes", bytes.NewReader(payload))
	rec := httptest.NewRecorder()
	routesHandler(router)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	configs := router.CurrentRoutes()
	if len(configs) != 1 || configs[0].Prefix != "/doubts" {
		t.Errorf("route table should have been replaced, got %+v", configs)
	}
}

func TestRoutesHandlerPostRejectsEmptyList(t *testing.T) {
	router, err := NewRouter([]RouteConfig{{Prefix: "/users", Upstream: "http://user-service"}})
	if err != nil {
		t.Fatalf("failed to build router: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/internal/routes", bytes.NewReader([]byte("[]")))
	rec := httptest.NewRecorder()
	routesHandler(router)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for an empty route list, got %d", rec.Code)
	}
}

func TestRoutesHandlerPostRejectsInvalidUpstream(t *testing.T) {
	router, err := NewRouter([]RouteConfig{{Prefix: "/users", Upstream: "http://user-service"}})
	if err != nil {
		t.Fatalf("failed to build router: %v", err)
	}

	payload, _ := json.Marshal([]RouteConfig{{Prefix: "/doubts", Upstream: "://not-a-valid-url"}})
	req := httptest.NewRequest(http.MethodPost, "/internal/routes", bytes.NewReader(payload))
	rec := httptest.NewRecorder()
	routesHandler(router)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for an invalid upstream, got %d", rec.Code)
	}
}
