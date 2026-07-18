package main

import (
	"log"
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
			log.Fatalf("invalid ROUTES: %v", err)
		}
		return configs
	}

	// backward-compatible fallback: a single catch-all upstream
	if upstream := os.Getenv("UPSTREAM_URL"); upstream != "" {
		return []RouteConfig{{Prefix: "/", Upstream: upstream}}
	}

	return nil
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler)

	if configs := loadRouteConfigs(); len(configs) > 0 {
		router, err := NewRouter(configs)
		if err != nil {
			log.Fatalf("failed to build router: %v", err)
		}
		mux.Handle("/", router)
	}

	log.Printf("gateway-core listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
