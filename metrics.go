package main

import (
	"fmt"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
)

// hand-rolled, dependency-free metrics registry emitting Prometheus text exposition format --
// no client library needed for a handful of counters, and per the coding-standards skill's rule
// on not blindly trusting AI-suggested dependencies, simplest is safest here.
var (
	requestsTotal       sync.Map // key: "method|path_prefix|status_class" -> *int64
	upstreamErrorsTotal sync.Map // key: upstream -> *int64
	rateLimitedTotal    int64
)

func statusClass(status int) string {
	switch {
	case status >= 500:
		return "5xx"
	case status >= 400:
		return "4xx"
	case status >= 300:
		return "3xx"
	default:
		return "2xx"
	}
}

func recordRequest(method, prefix string, status int) {
	key := method + "|" + prefix + "|" + statusClass(status)
	actual, _ := requestsTotal.LoadOrStore(key, new(int64))
	atomic.AddInt64(actual.(*int64), 1)
}

func recordUpstreamError(upstream string) {
	actual, _ := upstreamErrorsTotal.LoadOrStore(upstream, new(int64))
	atomic.AddInt64(actual.(*int64), 1)
}

func recordRateLimited() {
	atomic.AddInt64(&rateLimitedTotal, 1)
}

// metricsHandler backs GET /internal/metrics -- gated the same as /internal/log-level, this is
// operational data, not something to expose to the public internet.
func metricsHandler(breakers *CircuitBreakerRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")

		fmt.Fprintln(w, "# HELP gateway_requests_total Total requests handled, by method/route prefix/status class")
		fmt.Fprintln(w, "# TYPE gateway_requests_total counter")
		var reqKeys []string
		requestsTotal.Range(func(k, _ any) bool { reqKeys = append(reqKeys, k.(string)); return true })
		sort.Strings(reqKeys)
		for _, k := range reqKeys {
			v, _ := requestsTotal.Load(k)
			parts := splitMetricKey(k)
			fmt.Fprintf(w, "gateway_requests_total{method=%q,prefix=%q,status_class=%q} %d\n",
				parts[0], parts[1], parts[2], atomic.LoadInt64(v.(*int64)))
		}

		fmt.Fprintln(w, "# HELP gateway_upstream_errors_total Upstream request failures (connection errors, not HTTP error statuses), by upstream")
		fmt.Fprintln(w, "# TYPE gateway_upstream_errors_total counter")
		var upKeys []string
		upstreamErrorsTotal.Range(func(k, _ any) bool { upKeys = append(upKeys, k.(string)); return true })
		sort.Strings(upKeys)
		for _, k := range upKeys {
			v, _ := upstreamErrorsTotal.Load(k)
			fmt.Fprintf(w, "gateway_upstream_errors_total{upstream=%q} %d\n", k, atomic.LoadInt64(v.(*int64)))
		}

		fmt.Fprintln(w, "# HELP gateway_rate_limited_total Requests rejected for exceeding the rate limit")
		fmt.Fprintln(w, "# TYPE gateway_rate_limited_total counter")
		fmt.Fprintf(w, "gateway_rate_limited_total %d\n", atomic.LoadInt64(&rateLimitedTotal))

		fmt.Fprintln(w, "# HELP gateway_circuit_breaker_state Circuit breaker state per upstream (0=closed, 1=half-open, 2=open)")
		fmt.Fprintln(w, "# TYPE gateway_circuit_breaker_state gauge")
		for upstream, state := range breakers.Snapshot() {
			fmt.Fprintf(w, "gateway_circuit_breaker_state{upstream=%q,state=%q} %d\n", upstream, state, circuitStateValue(state))
		}
	}
}

func circuitStateValue(state string) int {
	switch state {
	case "open":
		return 2
	case "half-open":
		return 1
	default:
		return 0
	}
}

func splitMetricKey(key string) [3]string {
	var out [3]string
	idx := 0
	start := 0
	for i := 0; i < len(key) && idx < 2; i++ {
		if key[i] == '|' {
			out[idx] = key[start:i]
			idx++
			start = i + 1
		}
	}
	out[2] = key[start:]
	return out
}
