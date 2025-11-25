package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubEventsService struct {
	listFn func(ctx context.Context, namespace, name string) ([]EventResponse, error)
}

func (s stubEventsService) ListEvents(ctx context.Context, namespace, name string) ([]EventResponse, error) {
	return s.listFn(ctx, namespace, name)
}

func TestEventsHandlerHappyPath(t *testing.T) {
	handler := EventsHandler{
		Service: stubEventsService{
			listFn: func(_ context.Context, ns, name string) ([]EventResponse, error) {
				return []EventResponse{{Type: "Normal", Reason: "Created", Message: ns + "/" + name}}, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances/ns1/demo/events", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestEventsHandlerNamespaceDenied(t *testing.T) {
	handler := EventsHandler{
		AllowedNamespaces: []string{"allowed"},
		Service: stubEventsService{
			listFn: func(_ context.Context, _, _ string) ([]EventResponse, error) {
				t.Fatalf("should not be called")
				return nil, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances/denied/demo/events", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestEventsHandlerInvalidPath(t *testing.T) {
	handler := EventsHandler{
		Service: stubEventsService{
			listFn: func(_ context.Context, _, _ string) ([]EventResponse, error) {
				t.Fatalf("should not be called")
				return nil, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances/ns1/events", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
