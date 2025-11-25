package kube

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deicod/convex-operator-api/internal/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestFactoryUsesInClusterConfig(t *testing.T) {
	var (
		inClusterCalled    bool
		outOfClusterCalled bool
		newClientCalled    bool
	)

	f := Factory{
		inClusterConfig: func() (*rest.Config, error) {
			inClusterCalled = true
			return &rest.Config{Host: "in-cluster"}, nil
		},
		outOfClusterConfig: func(_, _ string) (*rest.Config, error) {
			outOfClusterCalled = true
			return nil, nil
		},
		newClient: func(cfg *rest.Config, opts client.Options) (client.WithWatch, error) {
			newClientCalled = true
			if cfg.Host != "in-cluster" {
				t.Fatalf("expected in-cluster config, got %q", cfg.Host)
			}
			return fake.NewClientBuilder().WithScheme(opts.Scheme).Build(), nil
		},
	}

	cli, restCfg, err := f.Client(config.Config{Mode: config.ModeInCluster})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cli == nil {
		t.Fatalf("expected client instance")
	}
	if restCfg == nil {
		t.Fatalf("expected rest config")
	}
	if !inClusterCalled {
		t.Fatalf("expected in-cluster config to be used")
	}
	if outOfClusterCalled {
		t.Fatalf("expected out-of-cluster config not to be used")
	}
	if !newClientCalled {
		t.Fatalf("expected client to be constructed")
	}
}

func TestFactoryUsesOutOfClusterConfig(t *testing.T) {
	var (
		receivedPath    string
		receivedContext string
	)

	f := Factory{
		inClusterConfig: func() (*rest.Config, error) {
			t.Fatalf("in-cluster config should not be used")
			return nil, nil
		},
		outOfClusterConfig: func(path, kubeContext string) (*rest.Config, error) {
			receivedPath = path
			receivedContext = kubeContext
			return &rest.Config{Host: "out-of-cluster"}, nil
		},
		newClient: func(cfg *rest.Config, opts client.Options) (client.WithWatch, error) {
			if cfg.Host != "out-of-cluster" {
				t.Fatalf("expected out-of-cluster config, got %q", cfg.Host)
			}
			return fake.NewClientBuilder().WithScheme(opts.Scheme).Build(), nil
		},
	}

	cfg := config.Config{
		Mode:           config.ModeOutOfCluster,
		KubeconfigPath: "/tmp/kubeconfig",
		KubeContext:    "staging",
	}

	_, restCfg, err := f.Client(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedPath != "/tmp/kubeconfig" {
		t.Fatalf("expected kubeconfig path to propagate, got %q", receivedPath)
	}
	if receivedContext != "staging" {
		t.Fatalf("expected kubecontext to propagate, got %q", receivedContext)
	}
	if restCfg == nil || restCfg.Host != "out-of-cluster" {
		t.Fatalf("expected rest config with host, got %+v", restCfg)
	}
}

func TestFactoryPropagatesConfigError(t *testing.T) {
	f := Factory{
		inClusterConfig: func() (*rest.Config, error) {
			return nil, errors.New("boom")
		},
		outOfClusterConfig: func(_, _ string) (*rest.Config, error) {
			return nil, nil
		},
		newClient: func(cfg *rest.Config, opts client.Options) (client.WithWatch, error) {
			_ = cfg
			_ = opts
			return nil, nil
		},
	}

	_, _, err := f.Client(config.Config{Mode: config.ModeInCluster})
	if err == nil {
		t.Fatalf("expected error to propagate")
	}
}

func TestAPIReadinessChecker(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r
		if r.URL.Path == "/readyz" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add corev1 to scheme: %v", err)
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	checker := APIReadinessChecker{
		Client:     fakeClient,
		RestConfig: &rest.Config{Host: srv.URL, TLSClientConfig: rest.TLSClientConfig{Insecure: true}},
	}

	if err := checker.Ready(context.Background()); err != nil {
		t.Fatalf("expected ready check to succeed, got %v", err)
	}
}

func TestAPIReadinessCheckerRequiresClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	checker := APIReadinessChecker{
		RestConfig: &rest.Config{Host: srv.URL, TLSClientConfig: rest.TLSClientConfig{Insecure: true}},
	}
	if err := checker.Ready(context.Background()); err == nil {
		t.Fatalf("expected error when client is nil")
	}
}

func TestAPIReadinessCheckerReadyzFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add corev1 to scheme: %v", err)
	}

	checker := APIReadinessChecker{
		Client:     fake.NewClientBuilder().WithScheme(scheme).Build(),
		RestConfig: &rest.Config{Host: srv.URL, TLSClientConfig: rest.TLSClientConfig{Insecure: true}},
	}

	if err := checker.Ready(context.Background()); err == nil {
		t.Fatalf("expected readyz failure to surface")
	}
}
