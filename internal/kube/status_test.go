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

func TestPodReadinessCounter(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	readyPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ready",
			Namespace: "ns1",
			Labels: map[string]string{
				"app.kubernetes.io/instance": "demo",
			},
		},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			}},
		},
	}
	notReadyPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "not-ready",
			Namespace: "ns1",
			Labels: map[string]string{
				"app.kubernetes.io/instance": "demo",
			},
		},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{{
				Type:   corev1.PodReady,
				Status: corev1.ConditionFalse,
			}},
		},
	}
	otherInstancePod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other",
			Namespace: "ns1",
			Labels: map[string]string{
				"app.kubernetes.io/instance": "someone-else",
			},
		},
	}

	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(readyPod, notReadyPod, otherInstancePod).
		Build()

	counter := PodReadinessCounter{Client: cli}
	ready, total, err := counter.CountReadyPods(context.Background(), "ns1", "demo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ready != 1 || total != 2 {
		t.Fatalf("unexpected counts ready=%d total=%d", ready, total)
	}
}
