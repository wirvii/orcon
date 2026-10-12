// Package httpapi is the HTTP adapter of Orcon. It translates HTTP requests
// into calls on area ports and area results into HTTP responses.
package httpapi

import "net/http"

// HealthPath is the route pattern the health endpoint is registered under.
const HealthPath = "GET /healthz"

// HealthHandler returns the liveness handler: it answers 200 with the body
// "ok" as long as the process can serve requests.
func HealthHandler() http.Handler {
	return http.HandlerFunc(healthz)
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	// The client may have gone away; there is nothing useful to do with the error.
	_, _ = w.Write([]byte("ok"))
}
