package kube

import (
	"context"

	"github.com/deicod/convex-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// InstanceStore wraps a controller-runtime client to manage ConvexInstance resources.
type InstanceStore struct {
	Client client.Client
}

// ListConvexInstances returns ConvexInstances in the given namespace or all namespaces when empty.
func (s InstanceStore) ListConvexInstances(ctx context.Context, namespace string) ([]v1alpha1.ConvexInstance, error) {
	list := &v1alpha1.ConvexInstanceList{}

	var opts []client.ListOption
	if namespace != "" {
		opts = append(opts, client.InNamespace(namespace))
	}

	if err := s.Client.List(ctx, list, opts...); err != nil {
		return nil, err
	}

	return list.Items, nil
}

// GetConvexInstance retrieves a ConvexInstance by namespace and name.
func (s InstanceStore) GetConvexInstance(ctx context.Context, namespace, name string) (*v1alpha1.ConvexInstance, error) {
	var obj v1alpha1.ConvexInstance
	if err := s.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &obj); err != nil {
		return nil, err
	}
	return &obj, nil
}

// CreateConvexInstance creates a new ConvexInstance.
func (s InstanceStore) CreateConvexInstance(ctx context.Context, obj *v1alpha1.ConvexInstance) error {
	return s.Client.Create(ctx, obj)
}

// UpdateConvexInstance updates an existing ConvexInstance.
func (s InstanceStore) UpdateConvexInstance(ctx context.Context, obj *v1alpha1.ConvexInstance) error {
	return s.Client.Update(ctx, obj)
}

// DeleteConvexInstance deletes a ConvexInstance by namespace and name.
func (s InstanceStore) DeleteConvexInstance(ctx context.Context, namespace, name string) error {
	obj := &v1alpha1.ConvexInstance{}
	obj.Namespace = namespace
	obj.Name = name
	return s.Client.Delete(ctx, obj)
}
