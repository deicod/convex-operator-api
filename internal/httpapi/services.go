package httpapi

import (
	"context"

	"github.com/deicod/convex-operator-api/internal/kube"
	"github.com/deicod/convex-operator/api/v1alpha1"
)

// StatusServiceAdapter wraps a kube.StatusReader.
type StatusServiceAdapter struct {
	Reader kube.StatusReader
}

// GetStatus fetches status for a ConvexInstance and converts to DTO.
func (a StatusServiceAdapter) GetStatus(ctx context.Context, namespace, name string) (*InstanceResponse, error) {
	ci, err := a.Reader.GetStatus(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	resp := FromConvexInstance(*ci)
	return &resp, nil
}

// EventsServiceAdapter wraps a kube.EventsReader.
type EventsServiceAdapter struct {
	Reader kube.EventsReader
}

// ListEvents retrieves events for a ConvexInstance and converts them.
func (a EventsServiceAdapter) ListEvents(ctx context.Context, namespace, name string) ([]EventResponse, error) {
	items, err := a.Reader.ListEvents(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	out := make([]EventResponse, 0, len(items))
	for _, ev := range items {
		resp := kube.EventToResponse(ev)
		out = append(out, EventResponse{
			Type:          resp.Type,
			Reason:        resp.Reason,
			Message:       resp.Message,
			LastTimestamp: resp.LastTimestamp,
		})
	}
	return out, nil
}

// UpgradeService defines upgrade behavior.
type UpgradeService interface {
	ApplyUpgrade(ctx context.Context, namespace, name, targetVersion, strategy string) (*v1alpha1.ConvexInstance, error)
}

// UpgradeServiceAdapter wraps kube.UpgradeSpecUpdater.
type UpgradeServiceAdapter struct {
	Updater kube.UpgradeSpecUpdater
}

// ApplyUpgrade updates version/strategy for a ConvexInstance.
func (a UpgradeServiceAdapter) ApplyUpgrade(ctx context.Context, namespace, name, targetVersion, strategy string) (*v1alpha1.ConvexInstance, error) {
	return a.Updater.ApplyUpgrade(ctx, namespace, name, targetVersion, strategy)
}
