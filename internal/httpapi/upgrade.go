package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
)

// UpgradeHandler serves POST /api/v1/instances/{namespace}/{name}/upgrade.
type UpgradeHandler struct {
	Service           UpgradeService
	AllowedNamespaces []string
}

// UpgradeRequest carries upgrade parameters.
type UpgradeRequest struct {
	TargetVersion string `json:"targetVersion"`
	Strategy      string `json:"strategy,omitempty"`
}

func (h UpgradeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ns, name, ok := parseUpgradePath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if !namespaceAllowed(h.AllowedNamespaces, ns) {
		writeError(w, http.StatusForbidden, "namespace not allowed")
		return
	}

	limitBody(w, r)
	var req UpgradeRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		if bodyTooLarge(err) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if strings.TrimSpace(req.TargetVersion) == "" {
		writeError(w, http.StatusBadRequest, "targetVersion is required")
		return
	}
	if req.Strategy != "" && req.Strategy != "inPlace" && req.Strategy != "exportImport" {
		writeError(w, http.StatusBadRequest, "strategy must be one of: inPlace, exportImport")
		return
	}

	obj, err := h.Service.ApplyUpgrade(r.Context(), ns, name, req.TargetVersion, req.Strategy)
	if err != nil {
		writeKubeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, FromConvexInstance(*obj))
}

func parseUpgradePath(path string) (namespace, name string, ok bool) {
	const prefix = "/api/v1/instances/"
	const suffix = "/upgrade"
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
