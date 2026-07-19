package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret-value"

func signTestToken(t *testing.T, secret string, sub string, expiry time.Time) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub": sub,
		"exp": expiry.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign test token: %v", err)
	}
	return signed
}

func TestWithJWTAuthAllowsPublicPrefixWithoutHeader(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	handler := withJWTAuth(inner, testSecret, []string{"/auth"})

	req := httptest.NewRequest(http.MethodPost, "/auth/otp/send", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected public prefix request to reach the wrapped handler")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestWithJWTAuthAllowsHealthzWithoutHeader(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	handler := withJWTAuth(inner, testSecret, nil)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected /healthz to reach the wrapped handler without auth")
	}
}

func TestWithJWTAuthAllowsOptionsWithoutHeader(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	handler := withJWTAuth(inner, testSecret, nil)

	req := httptest.NewRequest(http.MethodOptions, "/users/me", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected OPTIONS preflight to reach the wrapped handler without auth")
	}
}

func TestWithJWTAuthRejectsMissingHeader(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called without a valid token")
	})
	handler := withJWTAuth(inner, testSecret, nil)

	req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if body := rec.Body.String(); body == "" {
		t.Fatal("expected an error body")
	}
}

func TestWithJWTAuthRejectsMalformedHeader(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called without a valid token")
	})
	handler := withJWTAuth(inner, testSecret, nil)

	req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
	req.Header.Set("Authorization", "NotBearer abc123")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestWithJWTAuthRejectsExpiredToken(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called with an expired token")
	})
	handler := withJWTAuth(inner, testSecret, nil)

	token := signTestToken(t, testSecret, "user-1", time.Now().Add(-time.Hour))

	req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestWithJWTAuthRejectsWrongSignature(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called with a wrongly-signed token")
	})
	handler := withJWTAuth(inner, testSecret, nil)

	token := signTestToken(t, "a-different-secret", "user-1", time.Now().Add(time.Hour))

	req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestWithJWTAuthAcceptsValidTokenAndInjectsUserID(t *testing.T) {
	var seenUserID string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUserID = r.Header.Get("X-User-Id")
		w.WriteHeader(http.StatusOK)
	})
	handler := withJWTAuth(inner, testSecret, nil)

	token := signTestToken(t, testSecret, "user-42", time.Now().Add(30*24*time.Hour))

	req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if seenUserID != "user-42" {
		t.Fatalf("expected X-User-Id header to be %q, got %q", "user-42", seenUserID)
	}
}
