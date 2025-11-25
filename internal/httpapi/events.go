package httpapi

import (
	"context"
	"net/http"
	"strings"
)

// EventsService fetches Events associated with a ConvexInstance.
type EventsService interface {
	ListEvents(ctx context.Context, namespace, name string) ([]EventResponse, error)
}

// EventResponse represents a trimmed Event payload.
type EventResponse struct {
	Type          string `json:"type"`
	Reason        string `json:"reason"`
	Message       string `json:"message"`
	LastTimestamp string `json:"lastTimestamp"`
}

// EventsHandler serves GET /api/v1/instances/{namespace}/{name}/events.
type EventsHandler struct {
	Service           EventsService
	AllowedNamespaces []string
}

func (h EventsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ns, name, ok := parseEventsPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if !namespaceAllowed(h.AllowedNamespaces, ns) {
		writeError(w, http.StatusForbidden, "namespace not allowed")
		return
	}

	events, err := h.Service.ListEvents(r.Context(), ns, name)
	if err != nil {
		writeKubeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, events)
}

func parseEventsPath(path string) (namespace, name string, ok bool) {
	const prefix = "/api/v1/instances/"
	const suffix = "/events"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return "", "", false
	}
	trimmed := strings.TrimPrefix(path, prefix)
	trimmed = strings.TrimSuffix(trimmed, suffix)
	trimmed = strings.TrimSuffix(trimmed, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
