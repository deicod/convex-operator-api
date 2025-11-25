package httpapi

import "net/http"

// NewMux wires HTTP routes for the API service.
func NewMux(ready ReadyChecker, instances InstanceService, status StatusService, events EventsService, upgrade UpgradeService, watch InstanceWatchProvider, allowed []string) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("GET /healthz", HealthHandler())
	mux.Handle("GET /readyz", ReadyHandler(ready))

	handler := NewInstanceHandler(instances, allowed)
	mux.Handle("GET /api/v1/instances", http.HandlerFunc(handler.handleList))
	mux.Handle("GET /api/v1/instances/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isCollectionPath(r.URL.Path) {
			handler.handleList(w, r)
			return
		}
		handler.handleGet(w, r)
	}))
	mux.Handle("POST /api/v1/instances", http.HandlerFunc(handler.handleCreate))
	mux.Handle("POST /api/v1/instances/", http.HandlerFunc(handler.handleCreate))
	mux.Handle("PUT /api/v1/instances/", http.HandlerFunc(handler.handlePut))
	mux.Handle("PATCH /api/v1/instances/", http.HandlerFunc(handler.handlePatch))
	mux.Handle("DELETE /api/v1/instances/", http.HandlerFunc(handler.handleDelete))
	mux.Handle("GET /api/v1/instances/{namespace}/{name}/status", StatusHandler{Service: status, AllowedNamespaces: allowed})
	mux.Handle("GET /api/v1/instances/{namespace}/{name}/events", EventsHandler{Service: events, AllowedNamespaces: allowed})
	mux.Handle("POST /api/v1/instances/{namespace}/{name}/upgrade", UpgradeHandler{Service: upgrade, AllowedNamespaces: allowed})
	mux.Handle("GET /api/v1/instances/watch", WatchHandler{Watcher: watch, AllowedNamespaces: allowed})

	return mux
}
