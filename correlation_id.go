package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

type correlationIDContextKey struct{}

const requestIDHeader = "X-Request-Id"

// withCorrelationID ensures every request carries a correlation id: reused
// from the incoming X-Request-Id header if present, otherwise generated. The
// id is set on the response header, injected into the request context (so
// withRequestLogging can log it and downstream code can read it), and left on
// the request so it gets forwarded to the upstream by the reverse proxy.
func withCorrelationID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(requestIDHeader)
		if requestID == "" {
			requestID = generateRequestID()
		}

		r.Header.Set(requestIDHeader, requestID)
		w.Header().Set(requestIDHeader, requestID)

		ctx := context.WithValue(r.Context(), correlationIDContextKey{}, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requestIDFromContext returns the correlation id stored in the context, or
// "" if none was set (e.g. in tests that call a handler directly).
func requestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(correlationIDContextKey{}).(string); ok {
		return id
	}
	return ""
}

func generateRequestID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand failing is effectively unheard-of on real systems;
		// fall back to a fixed marker rather than panicking mid-request.
		return "00000000000000000000000000000000"
	}
	return hex.EncodeToString(buf)
}
