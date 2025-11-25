package httpapi

import (
	"net/http"
	"strings"
)

// AuthMiddleware enforces a static bearer token when configured.
type AuthMiddleware struct {
	Token string
}

// Wrap applies the middleware around the provided handler.
// When Token is empty, the middleware is a no-op.
func (a AuthMiddleware) Wrap(next http.Handler) http.Handler {
	if strings.TrimSpace(a.Token) == "" {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(authHeader, prefix) || strings.TrimPrefix(authHeader, prefix) != a.Token {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// HealthBypassMiddleware skips auth for health endpoints to keep probes working.
type HealthBypassMiddleware struct {
	Underlying  http.Handler // unprotected handler (typically the raw mux)
	Protected   http.Handler // handler wrapped with auth
	HealthPaths []string
}

// ServeHTTP bypasses auth on health/ready paths.
func (m HealthBypassMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, p := range m.HealthPaths {
		if r.URL.Path == p {
			m.Underlying.ServeHTTP(w, r)
			return
		}
	}
	m.Protected.ServeHTTP(w, r)
}
