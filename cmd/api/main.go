// Command api boots the convex-operator-api HTTP server.
package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/deicod/convex-operator-api/internal/config"
	"github.com/deicod/convex-operator-api/internal/httpapi"
	"github.com/deicod/convex-operator-api/internal/kube"
)

func main() {
	cfg, err := config.Parse(os.Args[1:], os.Getenv)
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	kubeFactory := kube.NewFactory()
	kubeClient, restCfg, err := kubeFactory.Client(cfg)
	if err != nil {
		log.Fatalf("kube client error: %v", err)
	}

	readyChecker := kube.APIReadinessChecker{
		Client:     kubeClient,
		RestConfig: restCfg,
	}

	instanceStore := kube.InstanceStore{Client: kubeClient}
	mux := httpapi.NewMux(readyChecker, instanceStore, cfg.AllowedNamespaces)

	server := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           loggingMiddleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("starting convex-operator-api on %s (mode=%s, kubecontext=%s)", cfg.ListenAddress, cfg.Mode, cfg.KubeContext)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
