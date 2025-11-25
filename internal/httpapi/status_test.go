package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deicod/convex-operator-api/internal/kube"
	"github.com/deicod/convex-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type stubStatusService struct {
	getFn func(ctx context.Context, namespace, name string) (*InstanceResponse, error)
}

func (s stubStatusService) GetStatus(ctx context.Context, namespace, name string) (*InstanceResponse, error) {
	return s.getFn(ctx, namespace, name)
}

func TestStatusHandlerHappyPath(t *testing.T) {
	handler := StatusHandler{
		Service: stubStatusService{
			getFn: func(_ context.Context, ns, name string) (*InstanceResponse, error) {
				return &InstanceResponse{
					Metadata: ObjectMeta{Name: name, Namespace: ns},
					Status:   v1alpha1.ConvexInstanceStatus{Phase: "Ready"},
				}, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances/ns1/demo/status", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestStatusHandlerNamespaceDenied(t *testing.T) {
	handler := StatusHandler{
		AllowedNamespaces: []string{"allowed"},
		Service: stubStatusService{
			getFn: func(_ context.Context, ns, name string) (*InstanceResponse, error) {
				t.Fatalf("should not be called %s/%s", ns, name)
				return nil, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances/denied/demo/status", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestStatusHandlerInvalidPath(t *testing.T) {
	handler := StatusHandler{
		Service: stubStatusService{
			getFn: func(_ context.Context, _, _ string) (*InstanceResponse, error) {
				t.Fatalf("should not be called")
				return nil, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances/ns1/status", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestStatusHandlerIncludesMetrics(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)

	instance := &v1alpha1.ConvexInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "ns1"},
		Status:     v1alpha1.ConvexInstanceStatus{Phase: "Ready"},
	}
	readyPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "p1",
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
			Name:      "p2",
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

	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(instance, readyPod, notReadyPod).
		Build()

	handler := StatusHandler{
		Service: StatusServiceAdapter{
			Reader:       kube.StatusReader{Client: cli},
			ReadyCounter: kube.PodReadinessCounter{Client: cli},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances/ns1/demo/status", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp InstanceResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Metrics == nil {
		t.Fatalf("expected metrics to be populated")
	}
	if resp.Metrics.ReadyPods != 1 || resp.Metrics.TotalPods != 2 {
		t.Fatalf("unexpected metrics: %+v", resp.Metrics)
	}
}
