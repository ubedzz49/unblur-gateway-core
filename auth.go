package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const userIDContextKey contextKey = "userID"

// withJWTAuth verifies a bearer JWT (HS256, shared secret) on every request
// except public prefixes, the health check, and CORS preflight (OPTIONS never
// carries an auth header). On success it injects the verified user id as an
// X-User-Id header on the forwarded request so downstream services can trust
// a gateway-verified identity instead of re-verifying the JWT themselves.
func withJWTAuth(next http.Handler, secret string, publicPrefixes []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions || r.URL.Path == "/healthz" || isPublicPath(r.URL.Path, publicPrefixes) {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if authHeader == "" || !strings.HasPrefix(authHeader, prefix) || strings.TrimSpace(authHeader[len(prefix):]) == "" {
			writeAuthError(w, http.StatusUnauthorized, "missing or invalid authorization header")
			return
		}

		tokenString := strings.TrimSpace(authHeader[len(prefix):])

		claims := jwt.MapClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			writeAuthError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		sub, ok := claims["sub"].(string)
		if !ok || sub == "" {
			writeAuthError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		r.Header.Set("X-User-Id", sub)
		ctx := context.WithValue(r.Context(), userIDContextKey, sub)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func isPublicPath(path string, publicPrefixes []string) bool {
	for _, prefix := range publicPrefixes {
		if prefix != "" && strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func writeAuthError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
