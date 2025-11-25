package httpapi

import "net/http"

// NewMux wires HTTP routes for the API service.
func NewMux(ready ReadyChecker, instances InstanceService, allowed []string) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/healthz", HealthHandler())
	mux.Handle("/readyz", ReadyHandler(ready))

	instancesHandler := NewInstanceHandler(instances, allowed)
	mux.Handle("/api/v1/instances", instancesHandler)
	mux.Handle("/api/v1/instances/", instancesHandler)

	return mux
}
