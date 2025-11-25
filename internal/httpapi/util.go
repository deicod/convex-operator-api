package httpapi

import (
	"errors"
	"net/http"
)

const defaultMaxRequestBodyBytes = 1 << 20 // 1 MiB

var maxRequestBodyBytes int64 = defaultMaxRequestBodyBytes

func namespaceAllowed(allowed []string, ns string) bool {
	if len(allowed) == 0 {
		return true
	}
	if ns == "" {
		return false
	}
	for _, a := range allowed {
		if ns == a {
			return true
		}
	}
	return false
}

func limitBody(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
}

func bodyTooLarge(err error) bool {
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr)
}
