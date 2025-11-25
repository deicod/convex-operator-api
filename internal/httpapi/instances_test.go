package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deicod/convex-operator/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type stubInstanceService struct {
	listFn   func(ctx context.Context, namespace string) ([]v1alpha1.ConvexInstance, error)
	getFn    func(ctx context.Context, namespace, name string) (*v1alpha1.ConvexInstance, error)
	createFn func(ctx context.Context, obj *v1alpha1.ConvexInstance) error
	updateFn func(ctx context.Context, obj *v1alpha1.ConvexInstance) error
	deleteFn func(ctx context.Context, namespace, name string) error
}

func (s stubInstanceService) ListConvexInstances(ctx context.Context, namespace string) ([]v1alpha1.ConvexInstance, error) {
	return s.listFn(ctx, namespace)
}
func (s stubInstanceService) GetConvexInstance(ctx context.Context, namespace, name string) (*v1alpha1.ConvexInstance, error) {
	return s.getFn(ctx, namespace, name)
}
func (s stubInstanceService) CreateConvexInstance(ctx context.Context, obj *v1alpha1.ConvexInstance) error {
	return s.createFn(ctx, obj)
}
func (s stubInstanceService) UpdateConvexInstance(ctx context.Context, obj *v1alpha1.ConvexInstance) error {
	return s.updateFn(ctx, obj)
}
func (s stubInstanceService) DeleteConvexInstance(ctx context.Context, namespace, name string) error {
	return s.deleteFn(ctx, namespace, name)
}

func makeConvexInstance(name, ns string) v1alpha1.ConvexInstance {
	return v1alpha1.ConvexInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: v1alpha1.ConvexInstanceSpec{
			Environment: "dev",
			Version:     "1.0.0",
			Backend:     v1alpha1.BackendSpec{DB: v1alpha1.BackendDatabaseSpec{Engine: "postgres"}},
			Networking:  v1alpha1.NetworkingSpec{Host: "example.com"},
		},
	}
}

func TestListInstances(t *testing.T) {
	svc := stubInstanceService{
		listFn: func(_ context.Context, namespace string) ([]v1alpha1.ConvexInstance, error) {
			if namespace != "ns1" {
				t.Fatalf("expected namespace filter ns1, got %s", namespace)
			}
			return []v1alpha1.ConvexInstance{makeConvexInstance("a", "ns1")}, nil
		},
	}

	handler := NewInstanceHandler(svc, []string{"ns1"})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances?namespace=ns1", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp InstancesListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].Metadata.Name != "a" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestGetInstance(t *testing.T) {
	svc := stubInstanceService{
		getFn: func(_ context.Context, namespace, name string) (*v1alpha1.ConvexInstance, error) {
			if namespace != "ns1" || name != "demo" {
				t.Fatalf("unexpected get args %s/%s", namespace, name)
			}
			obj := makeConvexInstance(name, namespace)
			return &obj, nil
		},
	}

	handler := NewInstanceHandler(svc, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances/ns1/demo", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestCreateInstance(t *testing.T) {
	var created *v1alpha1.ConvexInstance
	svc := stubInstanceService{
		createFn: func(_ context.Context, obj *v1alpha1.ConvexInstance) error {
			created = obj
			return nil
		},
	}

	reqBody := `{
		"metadata": {"name": "demo", "namespace": "ns1"},
		"spec": {"environment": "dev", "version": "1.0.0", "backend": {"db": {"engine": "postgres"}}, "networking": {"host": "example.com"}}
	}`
	handler := NewInstanceHandler(svc, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/instances", bytes.NewBufferString(reqBody))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	if created == nil || created.Name != "demo" || created.Namespace != "ns1" {
		t.Fatalf("expected object to be created, got %+v", created)
	}
}

func TestPutInstanceReplacesSpec(t *testing.T) {
	existing := makeConvexInstance("demo", "ns1")
	svc := stubInstanceService{
		getFn: func(_ context.Context, _ string, _ string) (*v1alpha1.ConvexInstance, error) {
			obj := existing
			return &obj, nil
		},
		updateFn: func(_ context.Context, obj *v1alpha1.ConvexInstance) error {
			if obj.Spec.Version != "2.0.0" {
				t.Fatalf("expected version replaced, got %s", obj.Spec.Version)
			}
			return nil
		},
	}

	body := `{
		"metadata": {"name": "ignored", "namespace": "ignored"},
		"spec": {"environment": "dev", "version": "2.0.0", "backend": {"db": {"engine": "postgres"}}, "networking": {"host": "example.com"}}
	}`
	handler := NewInstanceHandler(svc, nil)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/instances/ns1/demo", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestPatchInstance(t *testing.T) {
	existing := makeConvexInstance("demo", "ns1")
	svc := stubInstanceService{
		getFn: func(_ context.Context, _ string, _ string) (*v1alpha1.ConvexInstance, error) {
			obj := existing
			return &obj, nil
		},
		updateFn: func(_ context.Context, obj *v1alpha1.ConvexInstance) error {
			if obj.Spec.Version != "2.0.0" {
				t.Fatalf("expected version patched, got %s", obj.Spec.Version)
			}
			return nil
		},
	}

	body := `{"version":"2.0.0"}`
	handler := NewInstanceHandler(svc, nil)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/instances/ns1/demo", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestDeleteInstance(t *testing.T) {
	var deleted bool
	svc := stubInstanceService{
		deleteFn: func(_ context.Context, _ string, _ string) error {
			deleted = true
			return nil
		},
	}
	handler := NewInstanceHandler(svc, []string{"ns1"})
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/instances/ns1/demo", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if !deleted {
		t.Fatalf("expected delete to be called")
	}
}

func TestNamespaceNotAllowed(t *testing.T) {
	svc := stubInstanceService{
		getFn: func(_ context.Context, _ string, _ string) (*v1alpha1.ConvexInstance, error) {
			return nil, nil
		},
	}
	handler := NewInstanceHandler(svc, []string{"allowed"})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances/denied/demo", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestNotFoundMapping(t *testing.T) {
	svc := stubInstanceService{
		getFn: func(_ context.Context, _ string, _ string) (*v1alpha1.ConvexInstance, error) {
			return nil, apierrors.NewNotFound(schema.GroupResource{Group: "convex.icod.de", Resource: "convexinstances"}, "missing")
		},
	}
	handler := NewInstanceHandler(svc, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances/ns1/missing", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestBadJSONReturns400(t *testing.T) {
	svc := stubInstanceService{
		createFn: func(_ context.Context, _ *v1alpha1.ConvexInstance) error { return nil },
	}
	handler := NewInstanceHandler(svc, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/instances", bytes.NewBufferString("{"))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestServiceErrorMapsTo500(t *testing.T) {
	svc := stubInstanceService{
		listFn: func(_ context.Context, _ string) ([]v1alpha1.ConvexInstance, error) {
			return nil, errors.New("boom")
		},
	}
	handler := NewInstanceHandler(svc, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}
