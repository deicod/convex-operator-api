package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubHandler struct {
	called bool
}

func (s *stubHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	s.called = true
	w.WriteHeader(http.StatusOK)
}

func TestAuthMiddlewareAllowsWhenTokenEmpty(t *testing.T) {
	handler := &stubHandler{}
	mw := AuthMiddleware{Token: ""}
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	mw.Wrap(handler).ServeHTTP(rec, req)
	if !handler.called {
		t.Fatalf("expected handler to be called when token is empty")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAuthMiddlewareRejectsMissingHeader(t *testing.T) {
	handler := &stubHandler{}
	mw := AuthMiddleware{Token: "secret"}
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	mw.Wrap(handler).ServeHTTP(rec, req)
	if handler.called {
		t.Fatalf("expected handler not to be called")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuthMiddlewareRejectsWrongToken(t *testing.T) {
	handler := &stubHandler{}
	mw := AuthMiddleware{Token: "secret"}
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Authorization", "Bearer nope")
	rec := httptest.NewRecorder()

	mw.Wrap(handler).ServeHTTP(rec, req)
	if handler.called {
		t.Fatalf("expected handler not to be called")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuthMiddlewareAllowsCorrectToken(t *testing.T) {
	handler := &stubHandler{}
	mw := AuthMiddleware{Token: "secret"}
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()

	mw.Wrap(handler).ServeHTTP(rec, req)
	if !handler.called {
		t.Fatalf("expected handler to be called")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestHealthBypassMiddlewareAllowsHealthWithoutToken(t *testing.T) {
	rawHandler := &stubHandler{}
	protected := AuthMiddleware{Token: "secret"}.Wrap(rawHandler)
	mw := HealthBypassMiddleware{
		Underlying:  rawHandler,
		Protected:   protected,
		HealthPaths: []string{"/healthz", "/readyz"},
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected health to bypass auth, got %d", rec.Code)
	}
}

func TestHealthBypassMiddlewareStillRequiresAuthForAPI(t *testing.T) {
	rawHandler := &stubHandler{}
	protected := AuthMiddleware{Token: "secret"}.Wrap(rawHandler)
	mw := HealthBypassMiddleware{
		Underlying:  rawHandler,
		Protected:   protected,
		HealthPaths: []string{"/healthz", "/readyz"},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances", nil)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected API call to require auth, got %d", rec.Code)
	}
}
