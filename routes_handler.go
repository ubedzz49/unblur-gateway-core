package main

import (
	"encoding/json"
	"net/http"
)

// routesHandler backs GET/POST /internal/routes -- lets an admin view or replace the gateway's
// live routing table without a redeploy (V9's "admin/config service for managing gateway
// routes"). POST replaces the entire table atomically, same shape as the ROUTES env var used at
// boot -- simplest mental model ("this is the routing table now"), not a partial add/remove API.
// A replacement is NOT persisted back to the ROUTES env var -- it only lives in-memory until the
// next restart, which then reverts to the env var's value. Making a change permanent still means
// updating the ECS task definition, same as before; this endpoint is for live operational
// changes (e.g. temporarily pointing a prefix at a canary upstream), not permanent config.
func routesHandler(router *Router) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(router.CurrentRoutes())

		case http.MethodPost:
			var configs []RouteConfig
			if err := json.NewDecoder(r.Body).Decode(&configs); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
				return
			}
			if len(configs) == 0 {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "routes must be a non-empty array"})
				return
			}
			if err := router.UpdateRoutes(configs); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			json.NewEncoder(w).Encode(router.CurrentRoutes())

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}
