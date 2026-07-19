package main

import (
	"log/slog"
	"net/http"
	"os"
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

func main() {
	initLogger()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler)

	if configs := loadRouteConfigs(); len(configs) > 0 {
		router, err := NewRouter(configs)
		if err != nil {
			slog.Error("failed to build router", "error", err)
			os.Exit(1)
		}
		mux.Handle("/", router)
	}

	handler := withRequestLogging(withCORS(mux, loadAllowedOrigins()))

	slog.Info("gateway-core starting", "port", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
