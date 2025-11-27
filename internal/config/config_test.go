package config

import (
	"reflect"
	"testing"
	"time"
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
	if !cfg.S3.Enabled {
		t.Fatalf("expected s3 provisioning enabled by default")
	}
	if cfg.S3.StorageClass != "rook-ceph-bucket" {
		t.Fatalf("unexpected s3 storage class: %q", cfg.S3.StorageClass)
	}
	if cfg.S3.SecretNameTemplate != "convex-{{name}}-s3" {
		t.Fatalf("unexpected s3 secret template: %q", cfg.S3.SecretNameTemplate)
	}
	if cfg.S3.EndpointScheme != "https" {
		t.Fatalf("unexpected s3 endpoint scheme: %q", cfg.S3.EndpointScheme)
	}
	if cfg.S3.SecretKeys.Endpoint != "endpoint" || cfg.S3.SecretKeys.Bucket != "bucket" || cfg.S3.SecretKeys.Region != "region" || cfg.S3.SecretKeys.AccessKeyID != "accessKeyId" || cfg.S3.SecretKeys.SecretAccessKey != "secretAccessKey" {
		t.Fatalf("unexpected s3 secret keys: %+v", cfg.S3.SecretKeys)
	}
	if cfg.S3.ProvisionTimeout != 2*time.Minute {
		t.Fatalf("unexpected s3 provision timeout: %v", cfg.S3.ProvisionTimeout)
	}
	if !cfg.CNPG.Enabled {
		t.Fatalf("expected cnpg provisioning enabled by default")
	}
	if cfg.CNPG.Host != "pg-rw.postgres.svc.cluster.local" || cfg.CNPG.Port != 5432 {
		t.Fatalf("unexpected cnpg host/port: %s:%d", cfg.CNPG.Host, cfg.CNPG.Port)
	}
	if cfg.CNPG.AdminSecret != "pg-rw-superuser" {
		t.Fatalf("unexpected cnpg admin secret: %q", cfg.CNPG.AdminSecret)
	}
	if cfg.CNPG.JobNamespace != "postgres" {
		t.Fatalf("unexpected cnpg job namespace: %q", cfg.CNPG.JobNamespace)
	}
	if cfg.CNPG.JobServiceAccount != "convex-operator-api-db" {
		t.Fatalf("unexpected cnpg job serviceaccount: %q", cfg.CNPG.JobServiceAccount)
	}
	if cfg.CNPG.JobImage != "postgres:18-alpine" {
		t.Fatalf("unexpected cnpg job image: %q", cfg.CNPG.JobImage)
	}
	if cfg.CNPG.SSLMode != "require" {
		t.Fatalf("unexpected cnpg sslmode: %q", cfg.CNPG.SSLMode)
	}
	if cfg.CNPG.DBSecretNameTemplate != "convex-{{name}}-db" {
		t.Fatalf("unexpected cnpg db secret template: %q", cfg.CNPG.DBSecretNameTemplate)
	}
	if cfg.CNPG.JobTimeout != 2*time.Minute {
		t.Fatalf("unexpected cnpg job timeout: %v", cfg.CNPG.JobTimeout)
	}
}

func TestParseEnvironmentOverridesDefaults(t *testing.T) {
	env := map[string]string{
		"LISTEN_ADDRESS":           "127.0.0.1:9999",
		"MODE":                     ModeOutOfCluster,
		"KUBECONFIG":               "/tmp/kubeconfig",
		"KUBECONTEXT":              "dev",
		"ALLOWED_NAMESPACES":       "a,b , c",
		"CONVEX_API_AUTH_TOKEN":    "secret",
		"S3_STORAGE_CLASS":         "custom-class",
		"S3_SECRET_NAME_TEMPLATE":  "custom-{{name}}-s3",
		"S3_ENDPOINT_SCHEME":       "http",
		"S3_PROVISION_TIMEOUT":     "30s",
		"S3_SECRET_KEYS":           "endpoint=url,bucket=bkt,region=eu,accessKeyId=ak,secretAccessKey=sk",
		"ENABLE_S3_PROVISIONING":   "false",
		"CNPG_HOST":                "db.example",
		"CNPG_PORT":                "6543",
		"CNPG_ADMIN_SECRET":        "admin-secret",
		"CNPG_JOB_NAMESPACE":       "pg-jobs",
		"CNPG_JOB_SERVICE_ACCOUNT": "pg-job-sa",
		"CNPG_JOB_IMAGE":           "postgres:17-alpine",
		"CNPG_SSLMODE":             "verify-full",
		"DB_SECRET_NAME_TEMPLATE":  "db-{{name}}",
		"CNPG_JOB_TIMEOUT":         "45s",
		"ENABLE_DB_PROVISIONING":   "false",
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
	if cfg.S3.Enabled {
		t.Fatalf("expected s3 provisioning disabled via env")
	}
	if cfg.S3.StorageClass != "custom-class" {
		t.Fatalf("unexpected s3 storage class: %q", cfg.S3.StorageClass)
	}
	if cfg.S3.SecretNameTemplate != "custom-{{name}}-s3" {
		t.Fatalf("unexpected s3 secret template: %q", cfg.S3.SecretNameTemplate)
	}
	if cfg.S3.EndpointScheme != "http" {
		t.Fatalf("unexpected s3 endpoint scheme: %q", cfg.S3.EndpointScheme)
	}
	if cfg.S3.SecretKeys.Endpoint != "url" || cfg.S3.SecretKeys.Bucket != "bkt" || cfg.S3.SecretKeys.Region != "eu" || cfg.S3.SecretKeys.AccessKeyID != "ak" || cfg.S3.SecretKeys.SecretAccessKey != "sk" {
		t.Fatalf("unexpected s3 secret keys: %+v", cfg.S3.SecretKeys)
	}
	if cfg.S3.ProvisionTimeout != 30*time.Second {
		t.Fatalf("unexpected s3 provision timeout: %v", cfg.S3.ProvisionTimeout)
	}
	if cfg.CNPG.Enabled {
		t.Fatalf("expected cnpg provisioning disabled via env")
	}
	if cfg.CNPG.Host != "db.example" || cfg.CNPG.Port != 6543 {
		t.Fatalf("unexpected cnpg host/port: %s:%d", cfg.CNPG.Host, cfg.CNPG.Port)
	}
	if cfg.CNPG.AdminSecret != "admin-secret" {
		t.Fatalf("unexpected cnpg admin secret: %q", cfg.CNPG.AdminSecret)
	}
	if cfg.CNPG.JobNamespace != "pg-jobs" {
		t.Fatalf("unexpected cnpg job namespace: %q", cfg.CNPG.JobNamespace)
	}
	if cfg.CNPG.JobServiceAccount != "pg-job-sa" {
		t.Fatalf("unexpected cnpg job serviceaccount: %q", cfg.CNPG.JobServiceAccount)
	}
	if cfg.CNPG.JobImage != "postgres:17-alpine" {
		t.Fatalf("unexpected cnpg job image: %q", cfg.CNPG.JobImage)
	}
	if cfg.CNPG.SSLMode != "verify-full" {
		t.Fatalf("unexpected cnpg sslmode: %q", cfg.CNPG.SSLMode)
	}
	if cfg.CNPG.DBSecretNameTemplate != "db-{{name}}" {
		t.Fatalf("unexpected cnpg db secret template: %q", cfg.CNPG.DBSecretNameTemplate)
	}
	if cfg.CNPG.JobTimeout != 45*time.Second {
		t.Fatalf("unexpected cnpg job timeout: %v", cfg.CNPG.JobTimeout)
	}
}

func TestFlagsOverrideEnvironment(t *testing.T) {
	env := map[string]string{
		"LISTEN_ADDRESS":         ":1111",
		"MODE":                   ModeInCluster,
		"KUBECONTEXT":            "",
		"KUBECONFIG":             "/env/kubeconfig",
		"ALLOWED_NAMESPACES":     "ns-from-env",
		"CONVEX_API_AUTH_TOKEN":  "env-secret",
		"S3_STORAGE_CLASS":       "env-class",
		"S3_SECRET_KEYS":         "endpoint=env-endpoint",
		"ENABLE_S3_PROVISIONING": "false",
		"CNPG_HOST":              "env-host",
		"CNPG_PORT":              "6543",
		"ENABLE_DB_PROVISIONING": "false",
	}

	args := []string{
		"-listen=:2222",
		"-mode=out-of-cluster",
		"-kubeconfig=/flag/kubeconfig",
		"-kubecontext=prod",
		"-allowed-namespaces=ns-flag1,ns-flag2",
		"-auth-token=flag-secret",
		"-enable-s3-provisioning",
		"-s3-storage-class=flag-class",
		"-s3-secret-keys=endpoint=flag-endpoint,bucket=flag-bucket,accessKeyId=flag-ak,secretAccessKey=flag-sk,region=flag-region",
		"-cnpg-host=flag-host",
		"-cnpg-port=7777",
		"-cnpg-job-service-account=flag-sa",
		"-enable-db-provisioning",
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
	if !cfg.S3.Enabled {
		t.Fatalf("expected s3 provisioning enabled via flag override")
	}
	if cfg.S3.StorageClass != "flag-class" {
		t.Fatalf("unexpected s3 storage class: %q", cfg.S3.StorageClass)
	}
	if cfg.S3.SecretKeys.Endpoint != "flag-endpoint" || cfg.S3.SecretKeys.Bucket != "flag-bucket" || cfg.S3.SecretKeys.Region != "flag-region" || cfg.S3.SecretKeys.AccessKeyID != "flag-ak" || cfg.S3.SecretKeys.SecretAccessKey != "flag-sk" {
		t.Fatalf("unexpected s3 secret keys: %+v", cfg.S3.SecretKeys)
	}
	if !cfg.CNPG.Enabled {
		t.Fatalf("expected cnpg provisioning enabled via flag override")
	}
	if cfg.CNPG.Host != "flag-host" || cfg.CNPG.Port != 7777 {
		t.Fatalf("unexpected cnpg host/port: %s:%d", cfg.CNPG.Host, cfg.CNPG.Port)
	}
	if cfg.CNPG.JobServiceAccount != "flag-sa" {
		t.Fatalf("unexpected cnpg job serviceaccount: %q", cfg.CNPG.JobServiceAccount)
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

func TestInvalidS3TimeoutEnv(t *testing.T) {
	env := map[string]string{"S3_PROVISION_TIMEOUT": "not-a-duration"}
	if _, err := Parse(nil, mapLookup(env)); err == nil {
		t.Fatalf("expected error for invalid s3 timeout env")
	}
}

func TestInvalidS3SecretKeys(t *testing.T) {
	if _, err := Parse([]string{"-s3-secret-keys=badpair"}, mapLookup(nil)); err == nil {
		t.Fatalf("expected error for invalid s3 secret key mapping")
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
