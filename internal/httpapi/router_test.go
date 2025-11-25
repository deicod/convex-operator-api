package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubReady struct {
	err error
}

func (s stubReady) Ready(_ context.Context) error {
	return s.err
}

func TestRoutesHealthz(t *testing.T) {
	mux := NewMux(nil)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRoutesReadyzReady(t *testing.T) {
	mux := NewMux(stubReady{})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRoutesReadyzNotReady(t *testing.T) {
	mux := NewMux(stubReady{err: errors.New("not ready")})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestRoutesInstancesPlaceholder(t *testing.T) {
	mux := NewMux(nil)

	paths := []string{
		"/api/v1/instances",
		"/api/v1/instances/ns/name",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotImplemented {
			t.Fatalf("path %s: expected 501, got %d", path, rec.Code)
		}
	}
}
