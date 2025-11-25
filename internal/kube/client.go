// Package kube manages Kubernetes client initialization and readiness checks.
package kube

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/deicod/convex-operator-api/internal/config"
	"github.com/deicod/convex-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Factory builds Kubernetes clients using either in-cluster or out-of-cluster configuration.
type Factory struct {
	inClusterConfig    func() (*rest.Config, error)
	outOfClusterConfig func(path, kubeContext string) (*rest.Config, error)
	newClient          func(cfg *rest.Config, opts client.Options) (client.Client, error)
}

// NewFactory returns a Factory wired to real Kubernetes configuration helpers.
func NewFactory() Factory {
	return Factory{
		inClusterConfig:    rest.InClusterConfig,
		outOfClusterConfig: loadOutOfClusterConfig,
		newClient:          client.New,
	}
}

func (f Factory) restConfig(cfg config.Config) (*rest.Config, error) {
	switch cfg.Mode {
	case config.ModeInCluster:
		return f.inClusterConfig()
	case config.ModeOutOfCluster:
		return f.outOfClusterConfig(cfg.KubeconfigPath, cfg.KubeContext)
	default:
		return nil, fmt.Errorf("unsupported mode %q", cfg.Mode)
	}
}

// Client builds a controller-runtime client based on the provided config and also returns the rest.Config used.
func (f Factory) Client(cfg config.Config) (client.Client, *rest.Config, error) {
	restCfg, err := f.restConfig(cfg)
	if err != nil {
		return nil, nil, err
	}

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, nil, err
	}
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		return nil, nil, err
	}

	cli, err := f.newClient(restCfg, client.Options{Scheme: scheme})
	if err != nil {
		return nil, nil, err
	}

	return cli, restCfg, nil
}

func loadOutOfClusterConfig(path, kubeContext string) (*rest.Config, error) {
	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: path}
	overrides := &clientcmd.ConfigOverrides{}
	if kubeContext != "" {
		overrides.CurrentContext = kubeContext
	}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).ClientConfig()
}

// APIReadinessChecker verifies the Kubernetes API is reachable.
type APIReadinessChecker struct {
	Client     client.Client
	RestConfig *rest.Config
}

// Ready satisfies httpapi.ReadyChecker.
func (c APIReadinessChecker) Ready(ctx context.Context) error {
	if c.RestConfig == nil {
		return fmt.Errorf("rest config not provided")
	}
	if err := pingReadyz(ctx, c.RestConfig); err != nil {
		return err
	}

	if c.Client == nil {
		return fmt.Errorf("kube client not initialized")
	}
	return nil
}

// pingReadyz performs an unauthenticated-ready check using the API server /readyz endpoint.
func pingReadyz(ctx context.Context, cfg *rest.Config) error {
	httpClient, err := rest.HTTPClientFor(cfg)
	if err != nil {
		return err
	}

	host := strings.TrimSuffix(cfg.Host, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, host+"/readyz", nil)
	if err != nil {
		return err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("apiserver /readyz returned %s", resp.Status)
	}

	return nil
}
