package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func withCapturedLogs(t *testing.T, fn func()) []map[string]any {
	t.Helper()
	var buf bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(original)

	fn()

	var lines []map[string]any
	for _, line := range bytes.Split(buf.Bytes(), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			t.Fatalf("failed to parse log line: %v", err)
		}
		lines = append(lines, entry)
	}
	return lines
}

func TestRequestLoggingLevelsByStatus(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantLevel  string
	}{
		{"success", http.StatusOK, "INFO"},
		{"client error", http.StatusNotFound, "WARN"},
		{"server error", http.StatusBadGateway, "ERROR"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler := withRequestLogging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
			}))

			logs := withCapturedLogs(t, func() {
				req := httptest.NewRequest(http.MethodGet, "/some/path", nil)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)
			})

			if len(logs) != 1 {
				t.Fatalf("expected exactly one log line, got %d", len(logs))
			}
			if logs[0]["level"] != tc.wantLevel {
				t.Fatalf("expected level %s, got %v", tc.wantLevel, logs[0]["level"])
			}
			if logs[0]["status"] != float64(tc.statusCode) {
				t.Fatalf("expected status %d in log fields, got %v", tc.statusCode, logs[0]["status"])
			}
		})
	}
}
