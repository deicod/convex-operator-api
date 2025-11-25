package kube

import (
	"context"
	"testing"

	"github.com/deicod/convex-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newInstance(name, namespace, env string) *v1alpha1.ConvexInstance {
	return &v1alpha1.ConvexInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.ConvexInstanceSpec{
			Environment: env,
			Version:     "v1",
			Backend: v1alpha1.BackendSpec{
				DB: v1alpha1.BackendDatabaseSpec{
					Engine: "postgres",
				},
			},
			Networking: v1alpha1.NetworkingSpec{
				Host: "example.com",
			},
		},
	}
}

func TestListConvexInstancesAllNamespaces(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add v1alpha1 scheme: %v", err)
	}

	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			newInstance("a", "ns1", "dev"),
			newInstance("b", "ns2", "prod"),
		).
		Build()

	store := InstanceStore{Client: cli}
	items, err := store.ListConvexInstances(context.Background(), "")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
}

func TestListConvexInstancesByNamespace(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)

	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			newInstance("a", "ns1", "dev"),
			newInstance("b", "ns2", "prod"),
		).
		Build()

	store := InstanceStore{Client: cli}
	items, err := store.ListConvexInstances(context.Background(), "ns1")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(items) != 1 || items[0].Name != "a" {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestGetConvexInstance(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)

	obj := newInstance("a", "ns1", "dev")

	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()
	store := InstanceStore{Client: cli}

	got, err := store.GetConvexInstance(context.Background(), "ns1", "a")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if got.Name != "a" || got.Namespace != "ns1" {
		t.Fatalf("unexpected object: %+v", got)
	}
}

func TestCreateUpdateDeleteConvexInstance(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)

	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	store := InstanceStore{Client: cli}

	obj := newInstance("a", "ns1", "dev")
	if err := store.CreateConvexInstance(context.Background(), obj); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	got, err := store.GetConvexInstance(context.Background(), "ns1", "a")
	if err != nil {
		t.Fatalf("get after create failed: %v", err)
	}
	if got.Spec.Environment != "dev" {
		t.Fatalf("unexpected spec: %+v", got.Spec)
	}

	got.Spec.Environment = "prod"
	if err := store.UpdateConvexInstance(context.Background(), got); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	updated, err := store.GetConvexInstance(context.Background(), "ns1", "a")
	if err != nil {
		t.Fatalf("get after update failed: %v", err)
	}
	if updated.Spec.Environment != "prod" {
		t.Fatalf("expected environment updated, got %s", updated.Spec.Environment)
	}

	if err := store.DeleteConvexInstance(context.Background(), "ns1", "a"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
}
