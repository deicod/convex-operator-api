package kube

import (
	"context"
	"time"

	"github.com/deicod/convex-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EventsReader fetches events related to ConvexInstances.
type EventsReader struct {
	Client client.Client
}

// ListEvents returns Events in the namespace filtered by ConvexInstance name.
func (r EventsReader) ListEvents(ctx context.Context, namespace, name string) ([]corev1.Event, error) {
	var list corev1.EventList
	if err := r.Client.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	filtered := make([]corev1.Event, 0, len(list.Items))
	for _, ev := range list.Items {
		if ev.InvolvedObject.Kind == "ConvexInstance" && ev.InvolvedObject.Name == name {
			filtered = append(filtered, ev)
			continue
		}
		// Fallback: match owner references.
		for _, owner := range ev.OwnerReferences {
			if owner.Kind == "ConvexInstance" && owner.Name == name {
				filtered = append(filtered, ev)
				break
			}
		}
	}
	return filtered, nil
}

// UpgradeSpecUpdater adjusts ConvexInstance for upgrades.
type UpgradeSpecUpdater struct {
	Client client.Client
}

// ApplyUpgrade updates version and optional strategy.
func (u UpgradeSpecUpdater) ApplyUpgrade(ctx context.Context, namespace, name, targetVersion, strategy string) (*v1alpha1.ConvexInstance, error) {
	var obj v1alpha1.ConvexInstance
	if err := u.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &obj); err != nil {
		return nil, err
	}

	obj.Spec.Version = targetVersion
	if strategy != "" {
		obj.Spec.Maintenance.UpgradeStrategy = strategy
	}

	if err := u.Client.Update(ctx, &obj); err != nil {
		return nil, err
	}
	return &obj, nil
}

// EventResponse is a lightweight event view.
type EventResponse struct {
	Type          string `json:"type"`
	Reason        string `json:"reason"`
	Message       string `json:"message"`
	LastTimestamp string `json:"lastTimestamp"`
}

// EventToResponse converts a corev1.Event to EventResponse.
func EventToResponse(ev corev1.Event) EventResponse {
	ts := ""
	if ev.LastTimestamp != (metav1.Time{}) {
		ts = ev.LastTimestamp.Format(time.RFC3339)
	} else if !ev.EventTime.IsZero() {
		ts = ev.EventTime.Format(time.RFC3339)
	}

	return EventResponse{
		Type:          ev.Type,
		Reason:        ev.Reason,
		Message:       ev.Message,
		LastTimestamp: ts,
	}
}
