package kube

import (
	"context"

	"github.com/deicod/convex-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// StatusReader fetches ConvexInstance status.
type StatusReader struct {
	Client client.Client
}

// GetStatus returns the ConvexInstance with status populated.
func (s StatusReader) GetStatus(ctx context.Context, namespace, name string) (*v1alpha1.ConvexInstance, error) {
	var obj v1alpha1.ConvexInstance
	if err := s.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &obj); err != nil {
		return nil, err
	}
	return &obj, nil
}
