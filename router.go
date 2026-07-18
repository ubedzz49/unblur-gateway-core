package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strings"
)

type RouteConfig struct {
	Prefix   string `json:"prefix"`
	Upstream string `json:"upstream"`
}

type route struct {
	prefix string
	proxy  *httputil.ReverseProxy
}

// Router picks the longest matching path prefix and forwards to that route's upstream.
type Router struct {
	routes []route
}

func NewRouter(configs []RouteConfig) (*Router, error) {
	routes := make([]route, 0, len(configs))
	for _, c := range configs {
		upstream, err := url.Parse(c.Upstream)
		if err != nil {
			return nil, fmt.Errorf("invalid upstream %q for prefix %q: %w", c.Upstream, c.Prefix, err)
		}
		routes = append(routes, route{prefix: c.Prefix, proxy: newReverseProxy(upstream)})
	}

	// longest prefix first so a more specific route always wins over a shorter one
	sort.Slice(routes, func(i, j int) bool {
		return len(routes[i].prefix) > len(routes[j].prefix)
	})

	return &Router{routes: routes}, nil
}

func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, route := range rt.routes {
		if strings.HasPrefix(r.URL.Path, route.prefix) {
			route.proxy.ServeHTTP(w, r)
			return
		}
	}
	http.NotFound(w, r)
}

func ParseRouteConfigs(raw string) ([]RouteConfig, error) {
	var configs []RouteConfig
	if err := json.Unmarshal([]byte(raw), &configs); err != nil {
		return nil, fmt.Errorf("invalid route config json: %w", err)
	}
	return configs, nil
}
