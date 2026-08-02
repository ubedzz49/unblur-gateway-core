package main

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

const (
	maxRetries     = 2
	retryBaseDelay = 50 * time.Millisecond
)

// retryingTransport retries a request on connection-level failure (the request never reached
// the upstream, so retrying is always safe regardless of method) and, for idempotent methods
// only (GET/HEAD/OPTIONS), also retries on a 502/503/504 response. Never retries a POST/PUT/
// PATCH/DELETE that already got a response -- the upstream may have already processed it (e.g.
// a payment collect call), and retrying a non-idempotent request that already landed is exactly
// the double-charge risk the coding-standards skill's idempotency-key rule exists to prevent.
// Exponential backoff between attempts (50ms, 100ms).
type retryingTransport struct {
	base http.RoundTripper
}

func isIdempotentMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

func (t *retryingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// the request body is a single-read stream -- buffer it once so every retry attempt can
	// replay it from the start, instead of sending an empty body on attempt 2+
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			return nil, err
		}
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(retryBaseDelay * time.Duration(1<<(attempt-1)))
		}

		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			req.ContentLength = int64(len(bodyBytes))
		}

		resp, err := t.base.RoundTrip(req)
		if err != nil {
			lastErr = err
			slog.Warn("upstream request failed, retrying", "attempt", attempt, "error", err)
			continue
		}

		if isIdempotentMethod(req.Method) && attempt < maxRetries &&
			(resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusGatewayTimeout) {
			resp.Body.Close()
			slog.Warn("upstream returned a retryable status, retrying", "attempt", attempt, "status", resp.StatusCode)
			continue
		}

		return resp, nil
	}
	return nil, lastErr
}

func newReverseProxy(upstream *url.URL) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	proxy.Transport = &retryingTransport{base: http.DefaultTransport}

	originalDirector := proxy.Director
	proxy.Director = func(r *http.Request) {
		originalDirector(r)
		r.Host = upstream.Host
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		slog.Error("upstream request failed", "upstream", upstream.String(), "path", r.URL.Path, "error", err)
		recordUpstreamError(upstream.String())
		w.WriteHeader(http.StatusBadGateway)
	}

	return proxy
}
