package main

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoggingMiddlewareDoesNotLogSensitiveHeaders(t *testing.T) {
	var buf bytes.Buffer
	origOutput := log.Writer()
	origFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(origOutput)
		log.SetFlags(origFlags)
	}()

	handler := loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances", nil)
	req.Header.Set("Authorization", "Bearer super-secret-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logged := buf.String()
	if strings.Contains(logged, "super-secret-token") || strings.Contains(logged, "Authorization") {
		t.Fatalf("log output should not contain sensitive headers: %q", logged)
	}
	if !strings.Contains(logged, "GET /api/v1/instances") {
		t.Fatalf("expected method and path in logs, got %q", logged)
	}
}
