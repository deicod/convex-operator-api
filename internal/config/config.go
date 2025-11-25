// Package config handles parsing configuration for the API service.
package config

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	// ModeInCluster uses in-cluster Kubernetes configuration.
	ModeInCluster = "in-cluster"
	// ModeOutOfCluster uses a kubeconfig provided on disk.
	ModeOutOfCluster = "out-of-cluster"
)

// Config holds process configuration derived from defaults, environment, and CLI flags.
type Config struct {
	ListenAddress     string
	Mode              string
	KubeconfigPath    string
	KubeContext       string
	AllowedNamespaces []string
	AuthToken         string
}

// Parse builds a Config from the provided args and environment lookup.
// Precedence: defaults < environment variables < flags.
func Parse(args []string, getenv func(string) string) (Config, error) {
	if getenv == nil {
		getenv = os.Getenv
	}

	cfg := Config{
		ListenAddress: ":8080",
		Mode:          ModeInCluster,
	}

	if v := getenv("LISTEN_ADDRESS"); v != "" {
		cfg.ListenAddress = v
	}
	if v := getenv("MODE"); v != "" {
		cfg.Mode = strings.ToLower(v)
	}
	kubeconfigEnv := getenv("KUBECONFIG")
	kubeContextEnv := getenv("KUBECONTEXT")
	allowedEnv := getenv("ALLOWED_NAMESPACES")
	authTokenEnv := getenv("CONVEX_API_AUTH_TOKEN")

	fs := flag.NewFlagSet("convex-operator-api", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	fs.StringVar(&cfg.ListenAddress, "listen", cfg.ListenAddress, "listen address")
	fs.StringVar(&cfg.Mode, "mode", cfg.Mode, "operation mode: in-cluster or out-of-cluster")
	fs.StringVar(&cfg.KubeconfigPath, "kubeconfig", kubeconfigEnv, "path to kubeconfig (out-of-cluster)")
	fs.StringVar(&cfg.KubeContext, "kubecontext", kubeContextEnv, "kubeconfig context override")
	fs.StringVar(&cfg.AuthToken, "auth-token", authTokenEnv, "static bearer token for API access (optional)")

	var allowed string
	fs.StringVar(&allowed, "allowed-namespaces", allowedEnv, "comma-separated list of allowed namespaces")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	cfg.Mode = strings.ToLower(cfg.Mode)
	cfg.AllowedNamespaces = splitAndClean(allowed)

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) validate() error {
	switch c.Mode {
	case ModeInCluster:
	case ModeOutOfCluster:
		if c.KubeconfigPath == "" {
			return fmt.Errorf("kubeconfig path is required in out-of-cluster mode")
		}
	default:
		return fmt.Errorf("invalid mode %q", c.Mode)
	}
	return nil
}

func splitAndClean(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
