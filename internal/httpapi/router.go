package httpapi

import "net/http"

// NewMux wires HTTP routes for the API service.
func NewMux(ready ReadyChecker) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/healthz", HealthHandler())
	mux.Handle("/readyz", ReadyHandler(ready))

	placeholder := notImplementedHandler()
	mux.Handle("/api/v1/instances", placeholder)
	mux.Handle("/api/v1/instances/", placeholder)

	return mux
}

func notImplementedHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r
		http.Error(w, http.StatusText(http.StatusNotImplemented), http.StatusNotImplemented)
	})
}
