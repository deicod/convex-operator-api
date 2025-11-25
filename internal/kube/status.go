package kube

import (
	"context"

	"github.com/deicod/convex-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
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

// PodReadinessCounter reports basic pod readiness metrics for an instance.
type PodReadinessCounter struct {
	Client client.Client
}

// CountReadyPods returns the number of ready pods and total pods for the instance.
// Pods are selected via the common instance label applied by the operator.
func (p PodReadinessCounter) CountReadyPods(ctx context.Context, namespace, name string) (ready, total int, err error) {
	var pods corev1.PodList
	if err := p.Client.List(ctx, &pods,
		client.InNamespace(namespace),
		client.MatchingLabels(map[string]string{"app.kubernetes.io/instance": name}),
	); err != nil {
		return 0, 0, err
	}

	for _, pod := range pods.Items {
		total++
		if isPodReady(pod) {
			ready++
		}
	}
	return ready, total, nil
}

func isPodReady(pod corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
