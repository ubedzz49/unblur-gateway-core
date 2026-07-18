package main

import (
	"log"
	"net/http"
	"net/url"
	"os"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler)

	// K3 replaces this with a dynamic route table -- one hardcoded upstream for now
	if upstreamRaw := os.Getenv("UPSTREAM_URL"); upstreamRaw != "" {
		upstream, err := url.Parse(upstreamRaw)
		if err != nil {
			log.Fatalf("invalid UPSTREAM_URL: %v", err)
		}
		mux.Handle("/", newReverseProxy(upstream))
	}

	log.Printf("gateway-core listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
