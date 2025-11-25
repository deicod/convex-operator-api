package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deicod/convex-operator/api/v1alpha1"
)

type stubUpgradeService struct {
	applyFn func(ctx context.Context, namespace, name, targetVersion, strategy string) (*v1alpha1.ConvexInstance, error)
}

func (s stubUpgradeService) ApplyUpgrade(ctx context.Context, namespace, name, targetVersion, strategy string) (*v1alpha1.ConvexInstance, error) {
	return s.applyFn(ctx, namespace, name, targetVersion, strategy)
}

func TestUpgradeHandlerHappyPath(t *testing.T) {
	handler := UpgradeHandler{
		Service: stubUpgradeService{
			applyFn: func(_ context.Context, ns, name, version, strategy string) (*v1alpha1.ConvexInstance, error) {
				if version != "2.0.0" || strategy != "exportImport" {
					t.Fatalf("unexpected params: %s %s", version, strategy)
				}
				obj := v1alpha1.ConvexInstance{}
				obj.Name = name
				obj.Namespace = ns
				obj.Spec.Version = version
				obj.Spec.Maintenance.UpgradeStrategy = strategy
				return &obj, nil
			},
		},
	}

	body := `{"targetVersion":"2.0.0","strategy":"exportImport"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/instances/ns1/demo/upgrade", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestUpgradeHandlerValidatesNamespace(t *testing.T) {
	handler := UpgradeHandler{
		AllowedNamespaces: []string{"allowed"},
		Service: stubUpgradeService{
			applyFn: func(_ context.Context, _, _, _, _ string) (*v1alpha1.ConvexInstance, error) {
				t.Fatalf("should not be called")
				return nil, nil
			},
		},
	}

	body := `{"targetVersion":"2.0.0"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/instances/denied/demo/upgrade", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestUpgradeHandlerRejectsUnknownFields(t *testing.T) {
	handler := UpgradeHandler{
		Service: stubUpgradeService{
			applyFn: func(_ context.Context, _, _, _, _ string) (*v1alpha1.ConvexInstance, error) {
				t.Fatalf("should not be called")
				return nil, nil
			},
		},
	}

	body := `{"targetVersion":"2.0.0","extra":"nope"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/instances/ns1/demo/upgrade", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
