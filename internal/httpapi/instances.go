package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/deicod/convex-operator/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
func NewInstanceHandler(svc InstanceService, allowed []string) http.Handler {
	return &InstanceHandler{
		Service:           svc,
		AllowedNamespaces: allowed,
	}
}

func (h *InstanceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if r.URL.Path == "/api/v1/instances" {
			h.handleList(w, r)
		} else {
			h.handleGet(w, r)
		}
	case http.MethodPost:
		h.handleCreate(w, r)
	case http.MethodPut:
		h.handlePut(w, r)
	case http.MethodPatch:
		h.handlePatch(w, r)
	case http.MethodDelete:
		h.handleDelete(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *InstanceHandler) handleList(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	if !h.namespaceAllowed(namespace) {
		writeError(w, http.StatusForbidden, "namespace not allowed")
		return
	}

	items, err := h.Service.ListConvexInstances(r.Context(), namespace)
	if err != nil {
		writeKubeError(w, err)
		return
	}

	var resp InstancesListResponse
	resp.Items = make([]InstanceResponse, 0, len(items))
	for _, item := range items {
		resp.Items = append(resp.Items, FromConvexInstance(item))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *InstanceHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	ns, name, ok := parseDetailPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if !h.namespaceAllowed(ns) {
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
	req, err := decodeRequest(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !h.namespaceAllowed(req.Metadata.Namespace) {
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

	if !h.namespaceAllowed(ns) {
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
	if !h.namespaceAllowed(ns) {
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
	if !h.namespaceAllowed(ns) {
		writeError(w, http.StatusForbidden, "namespace not allowed")
		return
	}

	if err := h.Service.DeleteConvexInstance(r.Context(), ns, name); err != nil {
		writeKubeError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *InstanceHandler) namespaceAllowed(ns string) bool {
	if len(h.AllowedNamespaces) == 0 {
		return true
	}
	if ns == "" {
		return false
	}
	for _, allowed := range h.AllowedNamespaces {
		if ns == allowed {
			return true
		}
	}
	return false
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
		"message": msg,
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeKubeError(w http.ResponseWriter, err error) {
	if apierrors.IsNotFound(err) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if apierrors.IsAlreadyExists(err) {
		writeError(w, http.StatusConflict, "already exists")
		return
	}
	if apierrors.IsForbidden(err) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	writeError(w, http.StatusInternalServerError, fmt.Sprintf("internal error: %v", err))
}
