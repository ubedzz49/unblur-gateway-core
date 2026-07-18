package main

import (
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

	return proxy
}
