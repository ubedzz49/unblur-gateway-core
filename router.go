package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strings"
	"sync"
)

type RouteConfig struct {
	Prefix   string `json:"prefix"`
	Upstream string `json:"upstream"`
}

type route struct {
	prefix   string
	upstream string
	proxy    *httputil.ReverseProxy
	breaker  *CircuitBreaker
}

// Router picks the longest matching path prefix and forwards to that route's upstream.
// Routes are mutable at runtime (see UpdateRoutes / GET,POST /internal/routes in main.go), not
// just fixed at boot from the ROUTES env var -- a config change no longer needs a redeploy.
type Router struct {
	mu       sync.RWMutex
	routes   []route
	breakers *CircuitBreakerRegistry
}

func buildRoutes(configs []RouteConfig, breakers *CircuitBreakerRegistry) ([]route, error) {
	routes := make([]route, 0, len(configs))
	for _, c := range configs {
		upstream, err := url.Parse(c.Upstream)
		if err != nil {
			return nil, fmt.Errorf("invalid upstream %q for prefix %q: %w", c.Upstream, c.Prefix, err)
		}
		routes = append(routes, route{
			prefix:   c.Prefix,
			upstream: c.Upstream,
			proxy:    newReverseProxy(upstream),
			breaker:  breakers.For(c.Upstream),
		})
	}

	// longest prefix first so a more specific route always wins over a shorter one
	sort.Slice(routes, func(i, j int) bool {
		return len(routes[i].prefix) > len(routes[j].prefix)
	})
	return routes, nil
}

func NewRouter(configs []RouteConfig) (*Router, error) {
	return NewRouterWithBreakers(configs, NewCircuitBreakerRegistry())
}

func NewRouterWithBreakers(configs []RouteConfig, breakers *CircuitBreakerRegistry) (*Router, error) {
	routes, err := buildRoutes(configs, breakers)
	if err != nil {
		return nil, err
	}
	return &Router{routes: routes, breakers: breakers}, nil
}

// UpdateRoutes atomically swaps the live route table -- in-flight requests on the old table
// finish normally (they hold their own route/proxy reference), new requests see the new table.
func (rt *Router) UpdateRoutes(configs []RouteConfig) error {
	routes, err := buildRoutes(configs, rt.breakers)
	if err != nil {
		return err
	}
	rt.mu.Lock()
	rt.routes = routes
	rt.mu.Unlock()
	return nil
}

func (rt *Router) CurrentRoutes() []RouteConfig {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	configs := make([]RouteConfig, len(rt.routes))
	for i, r := range rt.routes {
		configs[i] = RouteConfig{Prefix: r.prefix, Upstream: r.upstream}
	}
	return configs
}

func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rt.mu.RLock()
	routes := rt.routes
	rt.mu.RUnlock()

	for _, rte := range routes {
		if !strings.HasPrefix(r.URL.Path, rte.prefix) {
			continue
		}

		if !rte.breaker.Allow() {
			recordUpstreamError(rte.upstream)
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error":"upstream temporarily unavailable, try again shortly"}`))
			recordRequest(r.Method, rte.prefix, http.StatusServiceUnavailable)
			return
		}

		captured := &statusCapturingWriter{ResponseWriter: w, status: http.StatusOK}
		rte.proxy.ServeHTTP(captured, r)

		if captured.status >= 500 {
			rte.breaker.RecordFailure()
		} else {
			rte.breaker.RecordSuccess()
		}
		recordRequest(r.Method, rte.prefix, captured.status)
		return
	}
	http.NotFound(w, r)
	recordRequest(r.Method, "unmatched", http.StatusNotFound)
}

func ParseRouteConfigs(raw string) ([]RouteConfig, error) {
	var configs []RouteConfig
	if err := json.Unmarshal([]byte(raw), &configs); err != nil {
		return nil, fmt.Errorf("invalid route config json: %w", err)
	}
	return configs, nil
}
