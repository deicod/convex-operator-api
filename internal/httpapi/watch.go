package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/deicod/convex-operator-api/internal/kube"
	"github.com/deicod/convex-operator/api/v1alpha1"
)

// InstanceWatchProvider starts watch streams for ConvexInstances.
type InstanceWatchProvider interface {
	Watch(ctx context.Context, namespace string) (kube.InstanceWatchStream, error)
}

// WatchHandler streams ConvexInstance watch events as SSE.
type WatchHandler struct {
	Watcher           InstanceWatchProvider
	AllowedNamespaces []string
}

// InstanceSummary is a condensed view of a ConvexInstance for streaming.
type InstanceSummary struct {
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	Phase        string `json:"phase,omitempty"`
	Version      string `json:"version,omitempty"`
	Environment  string `json:"environment,omitempty"`
	UpgradeHash  string `json:"upgradeHash,omitempty"`
	ObservedGen  int64  `json:"observedGeneration,omitempty"`
	APIURL       string `json:"apiUrl,omitempty"`
	DashboardURL string `json:"dashboardUrl,omitempty"`
}

// ServeHTTP handles GET /api/v1/instances/watch.
func (h WatchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	ns := r.URL.Query().Get("namespace")
	if ns != "" && !namespaceAllowed(h.AllowedNamespaces, ns) {
		writeError(w, http.StatusForbidden, "namespace not allowed")
		return
	}

	stream, err := h.Watcher.Watch(r.Context(), ns)
	if err != nil {
		writeKubeError(w, err)
		return
	}
	defer stream.Stop()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-stream.ResultChan():
			if !ok {
				return
			}
			if !namespaceAllowed(h.AllowedNamespaces, ev.Object.Namespace) {
				continue
			}
			summary := summarizeInstance(ev.Object)
			if err := writeSSE(w, ev.Type, summary); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func summarizeInstance(ci v1alpha1.ConvexInstance) InstanceSummary {
	return InstanceSummary{
		Name:         ci.Name,
		Namespace:    ci.Namespace,
		Phase:        ci.Status.Phase,
		Version:      ci.Spec.Version,
		Environment:  ci.Spec.Environment,
		UpgradeHash:  ci.Status.UpgradeHash,
		ObservedGen:  ci.Status.ObservedGeneration,
		APIURL:       ci.Status.Endpoints.APIURL,
		DashboardURL: ci.Status.Endpoints.DashboardURL,
	}
}

func writeSSE(w http.ResponseWriter, eventType string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "event: %s\n", eventType); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
		return err
	}
	return nil
}
