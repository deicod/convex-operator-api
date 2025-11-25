package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deicod/convex-operator-api/internal/kube"
	"github.com/deicod/convex-operator/api/v1alpha1"
)

type stubReady struct {
	err error
}

func (s stubReady) Ready(_ context.Context) error {
	return s.err
}

type noopStatus struct{}

func (noopStatus) GetStatus(ctx context.Context, namespace, name string) (*InstanceResponse, error) {
	_ = ctx
	_ = namespace
	_ = name
	return nil, nil
}

type noopEvents struct{}

func (noopEvents) ListEvents(ctx context.Context, namespace, name string) ([]EventResponse, error) {
	_ = ctx
	_ = namespace
	_ = name
	return nil, nil
}

type noopUpgrade struct{}

func (noopUpgrade) ApplyUpgrade(ctx context.Context, namespace, name, targetVersion, strategy string) (*v1alpha1.ConvexInstance, error) {
	_ = ctx
	_ = namespace
	_ = name
	_ = targetVersion
	_ = strategy
	return &v1alpha1.ConvexInstance{}, nil
}

type noopWatch struct{}

func (noopWatch) Watch(ctx context.Context, namespace string) (kube.InstanceWatchStream, error) {
	_ = ctx
	_ = namespace
	ch := make(chan kube.InstanceWatchEvent)
	close(ch)
	return &stubWatchStream{ch: ch}, nil
}

func TestRoutesHealthz(t *testing.T) {
	mux := NewMux(nil, stubInstanceService{}, noopStatus{}, noopEvents{}, noopUpgrade{}, noopWatch{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRoutesReadyzReady(t *testing.T) {
	mux := NewMux(stubReady{}, stubInstanceService{}, noopStatus{}, noopEvents{}, noopUpgrade{}, noopWatch{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRoutesReadyzNotReady(t *testing.T) {
	mux := NewMux(stubReady{err: errors.New("not ready")}, stubInstanceService{}, noopStatus{}, noopEvents{}, noopUpgrade{}, noopWatch{}, nil)

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
	mux := NewMux(nil, svc, noopStatus{}, noopEvents{}, noopUpgrade{}, noopWatch{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK && rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected handler to run, got %d", rec.Code)
	}
}
