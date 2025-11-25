// Package httpapi hosts HTTP handlers for the API service.
package httpapi

import (
	"context"
	"net/http"
)

// ReadyChecker reports whether the service is ready to receive traffic.
type ReadyChecker interface {
	Ready(ctx context.Context) error
}

// HealthHandler always reports the process is alive.
func HealthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
}

// ReadyHandler reports readiness based on the provided checker.
func ReadyHandler(checker ReadyChecker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if checker != nil {
			if err := checker.Ready(r.Context()); err != nil {
				http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	})
}
