package kube

import (
	"context"
	"testing"
	"time"

	"github.com/deicod/convex-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestEventsReaderFiltersByInstance(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	ev1 := corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "e1",
			Namespace: "ns1",
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "ConvexInstance",
			Namespace: "ns1",
			Name:      "demo",
		},
	}
	ev2 := corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "e2",
			Namespace: "ns1",
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "ConvexInstance",
			Namespace: "ns1",
			Name:      "other",
		},
	}

	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&ev1, &ev2).Build()
	reader := EventsReader{Client: cli}

	list, err := reader.ListEvents(context.Background(), "ns1", "demo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 1 || list[0].Name != "e1" {
		t.Fatalf("expected one matching event, got %+v", list)
	}
}

func TestEventToResponse(t *testing.T) {
	now := metav1.NewTime(time.Unix(1735689600, 0))
	ev := corev1.Event{
		Type:          "Normal",
		Reason:        "Created",
		Message:       "done",
		LastTimestamp: now,
	}

	resp := EventToResponse(ev)
	if resp.LastTimestamp == "" || resp.Type != "Normal" || resp.Reason != "Created" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestUpgradeSpecUpdater(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)

	obj := &v1alpha1.ConvexInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "ns1",
		},
		Spec: v1alpha1.ConvexInstanceSpec{
			Environment: "dev",
			Version:     "1.0.0",
			Backend:     v1alpha1.BackendSpec{DB: v1alpha1.BackendDatabaseSpec{Engine: "postgres"}},
			Networking:  v1alpha1.NetworkingSpec{Host: "example.com"},
		},
	}

	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()
	updater := UpgradeSpecUpdater{Client: cli}

	updated, err := updater.ApplyUpgrade(context.Background(), "ns1", "demo", "2.0.0", "exportImport")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Spec.Version != "2.0.0" || updated.Spec.Maintenance.UpgradeStrategy != "exportImport" {
		t.Fatalf("upgrade not applied: %+v", updated.Spec)
	}

	var fetched v1alpha1.ConvexInstance
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: "ns1", Name: "demo"}, &fetched); err != nil {
		t.Fatalf("failed to refetch: %v", err)
	}
	if fetched.Spec.Version != "2.0.0" {
		t.Fatalf("spec not persisted")
	}
}
