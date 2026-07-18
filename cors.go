package main

import (
	"net/http"
	"os"
	"strings"
)

const allowedMethods = "GET, POST, PATCH, PUT, DELETE, OPTIONS"
const allowedHeaders = "Content-Type, Authorization"

// loadAllowedOrigins reads CORS_ALLOWED_ORIGINS as a comma-separated list.
// An empty/unset value means "allow any origin" -- fine here since auth is a
// bearer token in a header, not a cookie, so there's no credential to leak.
func loadAllowedOrigins() []string {
	raw := os.Getenv("CORS_ALLOWED_ORIGINS")
	if raw == "" {
		return nil
	}
	origins := strings.Split(raw, ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}
	return origins
}

func isAllowedOrigin(origin string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, o := range allowed {
		if o == origin {
			return true
		}
	}
	return false
}

// withCORS handles preflight OPTIONS requests and adds CORS headers to every
// response so the frontend (a different origin -- different port on the same
// host today, a different subdomain once there's a real domain) can call this API.
func withCORS(next http.Handler, allowedOrigins []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && isAllowedOrigin(origin, allowedOrigins) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
			w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
