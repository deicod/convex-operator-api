package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deicod/convex-operator/api/v1alpha1"
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
