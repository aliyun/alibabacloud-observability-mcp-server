package server

import (
	"encoding/json"
	"net/http"
)

// healthResponse is the JSON body returned by the /health endpoint.
type healthResponse struct {
	Status string `json:"status"`
}

// healthHandler returns an http.HandlerFunc that responds with 200 OK
// and a JSON body {"status":"ok"}.
func healthHandler() http.HandlerFunc {
	body, _ := json.Marshal(healthResponse{Status: "ok"})
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}
}

// healthMux is an http.Handler that serves /health and delegates everything
// else to an inner handler. The inner handler can be set after construction
// to break circular dependencies with mcp-go server objects.
type healthMux struct {
	inner http.Handler
}

func (m *healthMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS headers for all responses.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, mcp-session-id, mcp-protocol-version")
	w.Header().Set("Access-Control-Expose-Headers", "mcp-session-id")

	// Handle preflight requests.
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.URL.Path == "/health" {
		healthHandler().ServeHTTP(w, r)
		return
	}
	if m.inner != nil {
		m.inner.ServeHTTP(w, r)
		return
	}
	http.NotFound(w, r)
}
