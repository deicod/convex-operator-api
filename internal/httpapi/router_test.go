package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deicod/convex-operator/api/v1alpha1"
)

type stubReady struct {
	err error
}

func (s stubReady) Ready(_ context.Context) error {
	return s.err
}

func TestRoutesHealthz(t *testing.T) {
	mux := NewMux(nil, stubInstanceService{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRoutesReadyzReady(t *testing.T) {
	mux := NewMux(stubReady{}, stubInstanceService{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRoutesReadyzNotReady(t *testing.T) {
	mux := NewMux(stubReady{err: errors.New("not ready")}, stubInstanceService{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestRoutesInstancesPlaceholder(t *testing.T) {
	svc := stubInstanceService{
		listFn: func(_ context.Context, namespace string) ([]v1alpha1.ConvexInstance, error) {
			_ = namespace
			return nil, nil
		},
	}
	mux := NewMux(nil, svc, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK && rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected handler to run, got %d", rec.Code)
	}
}
