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
	"github.com/deicod/convex-operator-api/internal/provision"
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
	var s3Prov *provision.S3Provisioner
	if cfg.S3.Enabled {
		s3Prov = &provision.S3Provisioner{
			Client: kubeClient,
			Config: cfg.S3,
		}
	}
	var dbProv *provision.CNPGProvisioner
	if cfg.CNPG.Enabled {
		dbProv = &provision.CNPGProvisioner{
			Client: kubeClient,
			Config: cfg.CNPG,
		}
	}
	instanceService := provision.LifecycleService{
		Store: instanceStore,
		S3:    s3Prov,
		DB:    dbProv,
		Log:   log.Default(),
	}
	statusService := httpapi.StatusServiceAdapter{
		Reader:       kube.StatusReader{Client: kubeClient},
		ReadyCounter: kube.PodReadinessCounter{Client: kubeClient},
	}
	eventsService := httpapi.EventsServiceAdapter{Reader: kube.EventsReader{Client: kubeClient}}
	upgradeService := httpapi.UpgradeServiceAdapter{Updater: kube.UpgradeSpecUpdater{Client: kubeClient}}
	instanceWatcher := kube.InstanceWatcher{Client: kubeClient}

	mux := httpapi.NewMux(readyChecker, instanceService, statusService, eventsService, upgradeService, instanceWatcher, cfg.AllowedNamespaces)

	protected := httpapi.AuthMiddleware{Token: cfg.AuthToken}.Wrap(mux)
	handler := httpapi.HealthBypassMiddleware{
		Underlying:  mux,
		Protected:   protected,
		HealthPaths: []string{"/healthz", "/readyz"},
	}

	server := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           loggingMiddleware(handler),
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
