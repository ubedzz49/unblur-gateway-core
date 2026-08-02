package main

import (
	"encoding/json"
	"net/http"
)

// withInternalToken gates a handler on the shared service-to-service secret, the same secret
// header every backend service checks for its own /internal/ routes -- never the end-user JWT.
func withInternalToken(next http.HandlerFunc, expectedToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Internal-Service-Token")
		if token == "" || token != expectedToken {
			http.Error(w, `{"error":"invalid internal service token"}`, http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

type logLevelBody struct {
	Level string `json:"level"`
}

// logLevelHandler backs GET/POST /internal/log-level -- runtime-mutable logging verbosity for
// the gateway itself, no redeploy needed. Same mechanism every backend service exposes.
func logLevelHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		json.NewEncoder(w).Encode(logLevelBody{Level: currentLogLevelName()})

	case http.MethodPost:
		var body logLevelBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
			return
		}
		if body.Level != "info" && body.Level != "debug" && body.Level != "error" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "level must be one of info, debug, error"})
			return
		}
		level := setLogLevel(body.Level)
		json.NewEncoder(w).Encode(logLevelBody{Level: level})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
