package main

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func newReverseProxy(upstream *url.URL) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(upstream)

	originalDirector := proxy.Director
	proxy.Director = func(r *http.Request) {
		originalDirector(r)
		r.Host = upstream.Host
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		slog.Error("upstream request failed", "upstream", upstream.String(), "path", r.URL.Path, "error", err)
		w.WriteHeader(http.StatusBadGateway)
	}

	return proxy
}
