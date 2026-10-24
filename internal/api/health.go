package api

import "net/http"

// Health is the liveness probe handler used by GET /health and the Docker HEALTHCHECK.
func Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"healthy"}`))
}
