package kube

import (
	"context"
	"errors"
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
		newClient: func(cfg *rest.Config, opts client.Options) (client.Client, error) {
			newClientCalled = true
			if cfg.Host != "in-cluster" {
				t.Fatalf("expected in-cluster config, got %q", cfg.Host)
			}
			return fake.NewClientBuilder().WithScheme(opts.Scheme).Build(), nil
		},
	}

	cli, err := f.Client(config.Config{Mode: config.ModeInCluster})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cli == nil {
		t.Fatalf("expected client instance")
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
		newClient: func(cfg *rest.Config, opts client.Options) (client.Client, error) {
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

	_, err := f.Client(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedPath != "/tmp/kubeconfig" {
		t.Fatalf("expected kubeconfig path to propagate, got %q", receivedPath)
	}
	if receivedContext != "staging" {
		t.Fatalf("expected kubecontext to propagate, got %q", receivedContext)
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
		newClient: func(cfg *rest.Config, opts client.Options) (client.Client, error) {
			_ = cfg
			_ = opts
			return nil, nil
		},
	}

	_, err := f.Client(config.Config{Mode: config.ModeInCluster})
	if err == nil {
		t.Fatalf("expected error to propagate")
	}
}

func TestAPIReadinessChecker(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add corev1 to scheme: %v", err)
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	checker := APIReadinessChecker{Client: fakeClient}

	if err := checker.Ready(context.Background()); err != nil {
		t.Fatalf("expected ready check to succeed, got %v", err)
	}
}

func TestAPIReadinessCheckerRequiresClient(t *testing.T) {
	checker := APIReadinessChecker{}
	if err := checker.Ready(context.Background()); err == nil {
		t.Fatalf("expected error when client is nil")
	}
}
