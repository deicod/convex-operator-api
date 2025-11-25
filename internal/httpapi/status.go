package httpapi

import (
	"context"
	"net/http"
	"strings"
)

// StatusService fetches ConvexInstance status.
type StatusService interface {
	GetStatus(ctx context.Context, namespace, name string) (*InstanceResponse, error)
}

// StatusHandler serves GET /api/v1/instances/{namespace}/{name}/status.
type StatusHandler struct {
	Service           StatusService
	AllowedNamespaces []string
}

// ServeHTTP handles status requests.
func (h StatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ns, name, ok := parseStatusPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if !namespaceAllowed(h.AllowedNamespaces, ns) {
		writeError(w, http.StatusForbidden, "namespace not allowed")
		return
	}

	resp, err := h.Service.GetStatus(r.Context(), ns, name)
	if err != nil {
		writeKubeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func parseStatusPath(path string) (namespace, name string, ok bool) {
	const prefix = "/api/v1/instances/"
	const suffix = "/status"
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
