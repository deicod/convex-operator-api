package config

import (
	"reflect"
	"testing"
)

func TestParseDefaults(t *testing.T) {
	cfg, err := Parse(nil, mapLookup(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ListenAddress != ":8080" {
		t.Fatalf("expected default listen address, got %q", cfg.ListenAddress)
	}
	if cfg.Mode != ModeInCluster {
		t.Fatalf("expected mode %q, got %q", ModeInCluster, cfg.Mode)
	}
	if cfg.KubeconfigPath != "" {
		t.Fatalf("expected empty kubeconfig, got %q", cfg.KubeconfigPath)
	}
	if len(cfg.AllowedNamespaces) != 0 {
		t.Fatalf("expected no allowed namespaces, got %v", cfg.AllowedNamespaces)
	}
	if cfg.AuthToken != "" {
		t.Fatalf("expected empty auth token by default")
	}
}

func TestParseEnvironmentOverridesDefaults(t *testing.T) {
	env := map[string]string{
		"LISTEN_ADDRESS":        "127.0.0.1:9999",
		"MODE":                  ModeOutOfCluster,
		"KUBECONFIG":            "/tmp/kubeconfig",
		"KUBECONTEXT":           "dev",
		"ALLOWED_NAMESPACES":    "a,b , c",
		"CONVEX_API_AUTH_TOKEN": "secret",
	}

	cfg, err := Parse(nil, mapLookup(env))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ListenAddress != "127.0.0.1:9999" {
		t.Fatalf("expected listen address from env, got %q", cfg.ListenAddress)
	}
	if cfg.Mode != ModeOutOfCluster {
		t.Fatalf("expected mode %q, got %q", ModeOutOfCluster, cfg.Mode)
	}
	if cfg.KubeconfigPath != "/tmp/kubeconfig" {
		t.Fatalf("expected kubeconfig path from env, got %q", cfg.KubeconfigPath)
	}
	if cfg.KubeContext != "dev" {
		t.Fatalf("expected kubecontext from env, got %q", cfg.KubeContext)
	}
	expectedNS := []string{"a", "b", "c"}
	if !reflect.DeepEqual(cfg.AllowedNamespaces, expectedNS) {
		t.Fatalf("unexpected allowed namespaces: %v", cfg.AllowedNamespaces)
	}
	if cfg.AuthToken != "secret" {
		t.Fatalf("expected auth token from env")
	}
}

func TestFlagsOverrideEnvironment(t *testing.T) {
	env := map[string]string{
		"LISTEN_ADDRESS":        ":1111",
		"MODE":                  ModeInCluster,
		"KUBECONFIG":            "/env/kubeconfig",
		"ALLOWED_NAMESPACES":    "ns-from-env",
		"CONVEX_API_AUTH_TOKEN": "env-secret",
	}

	args := []string{
		"-listen=:2222",
		"-mode=out-of-cluster",
		"-kubeconfig=/flag/kubeconfig",
		"-kubecontext=prod",
		"-allowed-namespaces=ns-flag1,ns-flag2",
		"-auth-token=flag-secret",
	}

	cfg, err := Parse(args, mapLookup(env))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ListenAddress != ":2222" {
		t.Fatalf("expected listen address from flags, got %q", cfg.ListenAddress)
	}
	if cfg.Mode != ModeOutOfCluster {
		t.Fatalf("expected mode %q, got %q", ModeOutOfCluster, cfg.Mode)
	}
	if cfg.KubeconfigPath != "/flag/kubeconfig" {
		t.Fatalf("expected kubeconfig from flag, got %q", cfg.KubeconfigPath)
	}
	if cfg.KubeContext != "prod" {
		t.Fatalf("expected kubecontext from flag, got %q", cfg.KubeContext)
	}
	expectedNS := []string{"ns-flag1", "ns-flag2"}
	if !reflect.DeepEqual(cfg.AllowedNamespaces, expectedNS) {
		t.Fatalf("unexpected allowed namespaces: %v", cfg.AllowedNamespaces)
	}
	if cfg.AuthToken != "flag-secret" {
		t.Fatalf("expected auth token from flag, got %q", cfg.AuthToken)
	}
}

func TestOutOfClusterRequiresKubeconfig(t *testing.T) {
	_, err := Parse([]string{"-mode=out-of-cluster"}, mapLookup(nil))
	if err == nil {
		t.Fatalf("expected error when kubeconfig missing")
	}
}

func TestInvalidMode(t *testing.T) {
	_, err := Parse([]string{"-mode=unknown"}, mapLookup(nil))
	if err == nil {
		t.Fatalf("expected error for invalid mode")
	}
}

func mapLookup(m map[string]string) func(string) string {
	return func(key string) string {
		if m == nil {
			return ""
		}
		return m[key]
	}
}
