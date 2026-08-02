package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRetryingTransportRetriesConnectionFailure(t *testing.T) {
	var attempts int32
	rt := &retryingTransport{base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		n := atomic.AddInt32(&attempts, 1)
		if n < 2 {
			return nil, io.ErrUnexpectedEOF
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
	})}

	req := httptest.NewRequest(http.MethodGet, "/seminars", nil)
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("expected the retry to succeed, got error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestRetryingTransportRetriesBadGatewayForGet(t *testing.T) {
	var attempts int32
	rt := &retryingTransport{base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		n := atomic.AddInt32(&attempts, 1)
		if n < 2 {
			return &http.Response{StatusCode: http.StatusBadGateway, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
	})}

	req := httptest.NewRequest(http.MethodGet, "/seminars", nil)
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected eventual 200, got %d", resp.StatusCode)
	}
}

func TestRetryingTransportDoesNotRetryPostOnBadGateway(t *testing.T) {
	var attempts int32
	rt := &retryingTransport{base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		atomic.AddInt32(&attempts, 1)
		return &http.Response{StatusCode: http.StatusBadGateway, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}

	req := httptest.NewRequest(http.MethodPost, "/internal/payments/collect", strings.NewReader(`{"amount":100}`))
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("expected the 502 to pass through unretried, got %d", resp.StatusCode)
	}
	if attempts != 1 {
		t.Errorf("a POST that already got a response must never be retried automatically -- got %d attempts", attempts)
	}
}

func TestRetryingTransportReplaysRequestBody(t *testing.T) {
	var bodiesSeen []string
	rt := &retryingTransport{base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(req.Body)
		bodiesSeen = append(bodiesSeen, string(b))
		if len(bodiesSeen) < 2 {
			return nil, io.ErrUnexpectedEOF
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
	})}

	req := httptest.NewRequest(http.MethodPost, "/internal/rooms", strings.NewReader(`{"type":"resolution"}`))
	_, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bodiesSeen) != 2 || bodiesSeen[0] != bodiesSeen[1] {
		t.Errorf("expected the same body on both attempts, got %+v", bodiesSeen)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
