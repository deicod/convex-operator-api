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

func TestStatusReaderGetStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)

	obj := &v1alpha1.ConvexInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "ns1",
		},
		Status: v1alpha1.ConvexInstanceStatus{
			Phase: "Ready",
		},
	}

	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()
	reader := StatusReader{Client: cli}

	got, err := reader.GetStatus(context.Background(), "ns1", "demo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status.Phase != "Ready" {
		t.Fatalf("expected status phase Ready, got %s", got.Status.Phase)
	}
}
