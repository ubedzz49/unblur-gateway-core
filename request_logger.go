package main

import (
	"log/slog"
	"net/http"
	"time"
)

type statusCapturingWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusCapturingWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// withRequestLogging logs one line per request: info for 2xx/3xx, warn for 4xx,
// error for 5xx, so a log level filter on the level alone tells you if
// something needs attention without reading every line.
func withRequestLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		captured := &statusCapturingWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(captured, r)

		fields := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", captured.status,
			"duration_ms", time.Since(start).Milliseconds(),
		}

		switch {
		case captured.status >= 500:
			slog.Error("request failed", fields...)
		case captured.status >= 400:
			slog.Warn("request rejected", fields...)
		default:
			slog.Info("request handled", fields...)
		}
	})
}
