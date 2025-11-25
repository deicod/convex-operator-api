package httpapi

import "net/http"

// NewMux wires HTTP routes for the API service.
func NewMux(ready ReadyChecker, instances InstanceService, allowed []string) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("GET /healthz", HealthHandler())
	mux.Handle("GET /readyz", ReadyHandler(ready))

	instancesHandler := NewInstanceHandler(instances, allowed)
	mux.Handle("GET /api/v1/instances", instancesHandler)
	mux.Handle("GET /api/v1/instances/", instancesHandler)
	mux.Handle("POST /api/v1/instances", instancesHandler)
	mux.Handle("POST /api/v1/instances/", instancesHandler)
	mux.Handle("PUT /api/v1/instances/", instancesHandler)
	mux.Handle("PATCH /api/v1/instances/", instancesHandler)
	mux.Handle("DELETE /api/v1/instances/", instancesHandler)

	return mux
}
