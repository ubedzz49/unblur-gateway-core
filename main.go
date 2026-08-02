package main

import (
	"log/slog"
	"net/http"
	"os"
	"strings"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func loadRouteConfigs() []RouteConfig {
	if raw := os.Getenv("ROUTES"); raw != "" {
		configs, err := ParseRouteConfigs(raw)
		if err != nil {
			slog.Error("invalid ROUTES config", "error", err)
			os.Exit(1)
		}
		slog.Info("loaded route config", "route_count", len(configs))
		return configs
	}

	// backward-compatible fallback: a single catch-all upstream
	if upstream := os.Getenv("UPSTREAM_URL"); upstream != "" {
		slog.Info("using single fallback upstream", "upstream", upstream)
		return []RouteConfig{{Prefix: "/", Upstream: upstream}}
	}

	slog.Warn("no ROUTES or UPSTREAM_URL configured, only /healthz will respond")
	return nil
}

// loadPublicRoutePrefixes reads PUBLIC_ROUTES as a comma-separated list of
// path prefixes that skip JWT verification, defaulting to /auth only.
// /expertise-options is also treated as public below: the user-service
// endpoint it proxies to has no auth check of its own (it backs the
// expertise picker shown during pre-login onboarding), so gating it here
// would just break that flow without adding real protection.
// /internal is public from the JWT layer's perspective too -- it has its own, separate gate
// (withInternalToken, the shared service-to-service secret), not a user JWT. Skipping JWT
// verification here isn't a hole: no user-facing route lives under /internal.
func loadPublicRoutePrefixes() []string {
	prefixes := []string{"/auth", "/expertise-options", "/internal"}
	if raw := os.Getenv("PUBLIC_ROUTES"); raw != "" {
		prefixes = nil
		for _, p := range strings.Split(raw, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				prefixes = append(prefixes, p)
			}
		}
	}
	return prefixes
}

func main() {
	initLogger()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		slog.Error("JWT_SECRET is not set; refusing to start unauthenticated")
		os.Exit(1)
	}

	// fail closed, same philosophy as every backend service's INTERNAL_SERVICE_TOKEN check -- an
	// unset token would otherwise mean /internal/log-level silently accepts anything
	internalServiceToken := os.Getenv("INTERNAL_SERVICE_TOKEN")
	if internalServiceToken == "" {
		slog.Error("INTERNAL_SERVICE_TOKEN is not set; refusing to start")
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler)
	mux.HandleFunc("/internal/log-level", withInternalToken(logLevelHandler, internalServiceToken))

	breakers := NewCircuitBreakerRegistry()
	mux.HandleFunc("/internal/metrics", withInternalToken(metricsHandler(breakers), internalServiceToken))

	if configs := loadRouteConfigs(); len(configs) > 0 {
		router, err := NewRouterWithBreakers(configs, breakers)
		if err != nil {
			slog.Error("failed to build router", "error", err)
			os.Exit(1)
		}
		mux.HandleFunc("/internal/routes", withInternalToken(routesHandler(router), internalServiceToken))
		mux.Handle("/", router)
	}

	// 120 requests/minute per client (user id once authenticated, else remote IP), refilled
	// continuously rather than in a fixed window -- generous enough for normal app usage, tight
	// enough to blunt a runaway retry loop or scripted abuse
	limiter := NewRateLimiter(120, 2)

	authenticated := withJWTAuth(mux, jwtSecret, loadPublicRoutePrefixes())
	handler := withCorrelationID(withRequestLogging(withCORS(withRateLimit(authenticated, limiter), loadAllowedOrigins())))

	slog.Info("gateway-core starting", "port", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
