// Package kube manages Kubernetes client initialization and readiness checks.
package kube

import (
	"context"
	"fmt"

	"github.com/deicod/convex-operator-api/internal/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
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
		inClusterConfig:    ctrl.GetConfig,
		outOfClusterConfig: loadOutOfClusterConfig,
		newClient:          client.New,
	}
}

// Client builds a controller-runtime client based on the provided config.
func (f Factory) Client(cfg config.Config) (client.Client, error) {
	var (
		restCfg *rest.Config
		err     error
	)

	switch cfg.Mode {
	case config.ModeInCluster:
		restCfg, err = f.inClusterConfig()
	case config.ModeOutOfCluster:
		restCfg, err = f.outOfClusterConfig(cfg.KubeconfigPath, cfg.KubeContext)
	default:
		return nil, fmt.Errorf("unsupported mode %q", cfg.Mode)
	}
	if err != nil {
		return nil, err
	}

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, err
	}

	return f.newClient(restCfg, client.Options{Scheme: scheme})
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
	Client client.Client
}

// Ready satisfies httpapi.ReadyChecker.
func (c APIReadinessChecker) Ready(ctx context.Context) error {
	if c.Client == nil {
		return fmt.Errorf("kube client not initialized")
	}
	return c.Client.List(ctx, &corev1.NamespaceList{}, client.Limit(1))
}
