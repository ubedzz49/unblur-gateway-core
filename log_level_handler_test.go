package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const testInternalToken = "test-internal-token"

func TestLogLevelHandlerRejectsWithoutToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/internal/log-level", nil)
	rec := httptest.NewRecorder()
	withInternalToken(logLevelHandler, testInternalToken)(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestLogLevelHandlerRejectsWrongToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/internal/log-level", nil)
	req.Header.Set("X-Internal-Service-Token", "wrong-token")
	rec := httptest.NewRecorder()
	withInternalToken(logLevelHandler, testInternalToken)(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestLogLevelHandlerGetReadsCurrentLevel(t *testing.T) {
	setLogLevel("info")
	req := httptest.NewRequest(http.MethodGet, "/internal/log-level", nil)
	req.Header.Set("X-Internal-Service-Token", testInternalToken)
	rec := httptest.NewRecorder()
	withInternalToken(logLevelHandler, testInternalToken)(rec, req)

	var body logLevelBody
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Level != "info" {
		t.Errorf("expected info, got %q", body.Level)
	}
}

func TestLogLevelHandlerPostChangesLevel(t *testing.T) {
	payload, _ := json.Marshal(logLevelBody{Level: "debug"})
	req := httptest.NewRequest(http.MethodPost, "/internal/log-level", bytes.NewReader(payload))
	req.Header.Set("X-Internal-Service-Token", testInternalToken)
	rec := httptest.NewRecorder()
	withInternalToken(logLevelHandler, testInternalToken)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := currentLogLevelName(); got != "debug" {
		t.Errorf("expected debug, got %q", got)
	}
	setLogLevel("info")
}

func TestLogLevelHandlerPostRejectsUnknownLevel(t *testing.T) {
	payload, _ := json.Marshal(logLevelBody{Level: "verbose"})
	req := httptest.NewRequest(http.MethodPost, "/internal/log-level", bytes.NewReader(payload))
	req.Header.Set("X-Internal-Service-Token", testInternalToken)
	rec := httptest.NewRecorder()
	withInternalToken(logLevelHandler, testInternalToken)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}
