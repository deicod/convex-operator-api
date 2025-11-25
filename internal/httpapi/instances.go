package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/deicod/convex-operator/api/v1alpha1"
)

// InstanceService defines CRUD interactions for ConvexInstances.
type InstanceService interface {
	ListConvexInstances(ctx context.Context, namespace string) ([]v1alpha1.ConvexInstance, error)
	GetConvexInstance(ctx context.Context, namespace, name string) (*v1alpha1.ConvexInstance, error)
	CreateConvexInstance(ctx context.Context, obj *v1alpha1.ConvexInstance) error
	UpdateConvexInstance(ctx context.Context, obj *v1alpha1.ConvexInstance) error
	DeleteConvexInstance(ctx context.Context, namespace, name string) error
}

// InstanceHandler serves ConvexInstance CRUD endpoints.
type InstanceHandler struct {
	Service           InstanceService
	AllowedNamespaces []string
}

// NewInstanceHandler constructs a handler for CRUD operations.
func NewInstanceHandler(svc InstanceService, allowed []string) *InstanceHandler {
	return &InstanceHandler{
		Service:           svc,
		AllowedNamespaces: allowed,
	}
}

func (h *InstanceHandler) handleList(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		h.handleListAllAllowed(w, r)
		return
	}

	if !namespaceAllowed(h.AllowedNamespaces, namespace) {
		writeError(w, http.StatusForbidden, "namespace not allowed")
		return
	}

	items, err := h.Service.ListConvexInstances(r.Context(), namespace)
	if err != nil {
		writeKubeError(w, err)
		return
	}

	writeInstances(w, items)
}

func (h *InstanceHandler) handleListAllAllowed(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var items []v1alpha1.ConvexInstance
	if len(h.AllowedNamespaces) == 0 {
		result, err := h.Service.ListConvexInstances(ctx, "")
		if err != nil {
			writeKubeError(w, err)
			return
		}
		items = result
	} else {
		for _, ns := range h.AllowedNamespaces {
			result, err := h.Service.ListConvexInstances(ctx, ns)
			if err != nil {
				writeKubeError(w, err)
				return
			}
			items = append(items, result...)
		}
	}

	writeInstances(w, items)
}

func (h *InstanceHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	ns, name, ok := parseDetailPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if !namespaceAllowed(h.AllowedNamespaces, ns) {
		writeError(w, http.StatusForbidden, "namespace not allowed")
		return
	}

	obj, err := h.Service.GetConvexInstance(r.Context(), ns, name)
	if err != nil {
		writeKubeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, FromConvexInstance(*obj))
}

func (h *InstanceHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	if !isCollectionPath(r.URL.Path) {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	req, err := decodeRequest(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !namespaceAllowed(h.AllowedNamespaces, req.Metadata.Namespace) {
		writeError(w, http.StatusForbidden, "namespace not allowed")
		return
	}

	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	obj := req.ToConvexInstance()
	if err := h.Service.CreateConvexInstance(r.Context(), &obj); err != nil {
		writeKubeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, FromConvexInstance(obj))
}

func (h *InstanceHandler) handlePut(w http.ResponseWriter, r *http.Request) {
	ns, name, ok := parseDetailPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	req, err := decodeRequest(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Metadata.Name = name
	req.Metadata.Namespace = ns

	if !namespaceAllowed(h.AllowedNamespaces, ns) {
		writeError(w, http.StatusForbidden, "namespace not allowed")
		return
	}
	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Ensure the resource exists before update.
	existing, err := h.Service.GetConvexInstance(r.Context(), ns, name)
	if err != nil {
		writeKubeError(w, err)
		return
	}

	existing.Spec = req.Spec
	if err := h.Service.UpdateConvexInstance(r.Context(), existing); err != nil {
		writeKubeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, FromConvexInstance(*existing))
}

func (h *InstanceHandler) handlePatch(w http.ResponseWriter, r *http.Request) {
	ns, name, ok := parseDetailPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if !namespaceAllowed(h.AllowedNamespaces, ns) {
		writeError(w, http.StatusForbidden, "namespace not allowed")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	existing, err := h.Service.GetConvexInstance(r.Context(), ns, name)
	if err != nil {
		writeKubeError(w, err)
		return
	}

	patchedSpec, err := ApplySpecPatch(existing.Spec, body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	req := InstanceRequest{
		Metadata: ObjectMeta{Name: name, Namespace: ns},
		Spec:     patchedSpec,
	}
	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	existing.Spec = patchedSpec
	if err := h.Service.UpdateConvexInstance(r.Context(), existing); err != nil {
		writeKubeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, FromConvexInstance(*existing))
}

func (h *InstanceHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	ns, name, ok := parseDetailPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if !namespaceAllowed(h.AllowedNamespaces, ns) {
		writeError(w, http.StatusForbidden, "namespace not allowed")
		return
	}

	if err := h.Service.DeleteConvexInstance(r.Context(), ns, name); err != nil {
		writeKubeError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func isCollectionPath(path string) bool {
	return path == "/api/v1/instances" || path == "/api/v1/instances/"
}

func parseDetailPath(path string) (namespace, name string, ok bool) {
	const prefix = "/api/v1/instances/"
	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}
	trimmed := strings.TrimPrefix(path, prefix)
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func decodeRequest(body io.Reader) (InstanceRequest, error) {
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	var req InstanceRequest
	if err := decoder.Decode(&req); err != nil {
		return InstanceRequest{}, err
	}
	return req, nil
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"code":    http.StatusText(status),
		"message": msg,
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeInstances(w http.ResponseWriter, items []v1alpha1.ConvexInstance) {
	var resp InstancesListResponse
	resp.Items = make([]InstanceResponse, 0, len(items))
	for _, item := range items {
		resp.Items = append(resp.Items, FromConvexInstance(item))
	}
	writeJSON(w, http.StatusOK, resp)
}
