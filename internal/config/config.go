// Package config handles parsing configuration for the API service.
package config

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
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
	S3                S3Config
	CNPG              CNPGConfig
}

// S3SecretKeys defines the key names expected in the canonical S3 secret.
type S3SecretKeys struct {
	Endpoint        string
	Bucket          string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
}

// S3Config captures S3 provisioning settings.
type S3Config struct {
	Enabled            bool
	StorageClass       string
	SecretNameTemplate string
	EndpointScheme     string
	SecretKeys         S3SecretKeys
	ProvisionTimeout   time.Duration
}

// CNPGConfig captures Postgres provisioning settings for a CNPG cluster.
type CNPGConfig struct {
	Enabled              bool
	Host                 string
	Port                 int
	AdminSecret          string
	JobNamespace         string
	JobImage             string
	SSLMode              string
	DBSecretNameTemplate string
	JobTimeout           time.Duration
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
		S3: S3Config{
			Enabled:            true,
			StorageClass:       "rook-ceph-bucket",
			SecretNameTemplate: "convex-{{name}}-s3",
			EndpointScheme:     "https",
			SecretKeys: S3SecretKeys{
				Endpoint:        "endpoint",
				Bucket:          "bucket",
				Region:          "region",
				AccessKeyID:     "accessKeyId",
				SecretAccessKey: "secretAccessKey",
			},
			ProvisionTimeout: 2 * time.Minute,
		},
		CNPG: CNPGConfig{
			Enabled:              true,
			Host:                 "pg-rw.postgres.svc.cluster.local",
			Port:                 5432,
			AdminSecret:          "pg-rw-superuser",
			JobNamespace:         "postgres",
			JobImage:             "postgres:18-alpine",
			SSLMode:              "require",
			DBSecretNameTemplate: "convex-{{name}}-db",
			JobTimeout:           2 * time.Minute,
		},
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
	s3StorageClassEnv := getenv("S3_STORAGE_CLASS")
	s3SecretTemplateEnv := getenv("S3_SECRET_NAME_TEMPLATE")
	s3EndpointSchemeEnv := getenv("S3_ENDPOINT_SCHEME")
	s3ProvisionTimeoutEnv := getenv("S3_PROVISION_TIMEOUT")
	s3SecretKeysEnv := getenv("S3_SECRET_KEYS")
	s3EnabledEnv := getenv("ENABLE_S3_PROVISIONING")
	cnpgHostEnv := getenv("CNPG_HOST")
	cnpgPortEnv := getenv("CNPG_PORT")
	cnpgAdminSecretEnv := getenv("CNPG_ADMIN_SECRET")
	cnpgJobNamespaceEnv := getenv("CNPG_JOB_NAMESPACE")
	cnpgJobImageEnv := getenv("CNPG_JOB_IMAGE")
	cnpgSSLModeEnv := getenv("CNPG_SSLMODE")
	cnpgDBSecretTemplateEnv := getenv("DB_SECRET_NAME_TEMPLATE")
	cnpgJobTimeoutEnv := getenv("CNPG_JOB_TIMEOUT")
	cnpgEnabledEnv := getenv("ENABLE_DB_PROVISIONING")

	if v := s3StorageClassEnv; v != "" {
		cfg.S3.StorageClass = v
	}
	if v := s3SecretTemplateEnv; v != "" {
		cfg.S3.SecretNameTemplate = v
	}
	if v := s3EndpointSchemeEnv; v != "" {
		cfg.S3.EndpointScheme = v
	}
	if v := s3ProvisionTimeoutEnv; v != "" {
		dur, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid S3_PROVISION_TIMEOUT: %w", err)
		}
		cfg.S3.ProvisionTimeout = dur
	}
	if v := s3EnabledEnv; v != "" {
		parsed, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid ENABLE_S3_PROVISIONING: %w", err)
		}
		cfg.S3.Enabled = parsed
	}

	if v := cnpgHostEnv; v != "" {
		cfg.CNPG.Host = v
	}
	if v := cnpgPortEnv; v != "" {
		port, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid CNPG_PORT: %w", err)
		}
		cfg.CNPG.Port = port
	}
	if v := cnpgAdminSecretEnv; v != "" {
		cfg.CNPG.AdminSecret = v
	}
	if v := cnpgJobNamespaceEnv; v != "" {
		cfg.CNPG.JobNamespace = v
	}
	if v := cnpgJobImageEnv; v != "" {
		cfg.CNPG.JobImage = v
	}
	if v := cnpgSSLModeEnv; v != "" {
		cfg.CNPG.SSLMode = v
	}
	if v := cnpgDBSecretTemplateEnv; v != "" {
		cfg.CNPG.DBSecretNameTemplate = v
	}
	if v := cnpgJobTimeoutEnv; v != "" {
		dur, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid CNPG_JOB_TIMEOUT: %w", err)
		}
		cfg.CNPG.JobTimeout = dur
	}
	if v := cnpgEnabledEnv; v != "" {
		parsed, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid ENABLE_DB_PROVISIONING: %w", err)
		}
		cfg.CNPG.Enabled = parsed
	}

	fs := flag.NewFlagSet("convex-operator-api", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	fs.StringVar(&cfg.ListenAddress, "listen", cfg.ListenAddress, "listen address")
	fs.StringVar(&cfg.Mode, "mode", cfg.Mode, "operation mode: in-cluster or out-of-cluster")
	fs.StringVar(&cfg.KubeconfigPath, "kubeconfig", kubeconfigEnv, "path to kubeconfig (out-of-cluster)")
	fs.StringVar(&cfg.KubeContext, "kubecontext", kubeContextEnv, "kubeconfig context override")
	fs.StringVar(&cfg.AuthToken, "auth-token", authTokenEnv, "static bearer token for API access (optional)")

	var allowed string
	fs.StringVar(&allowed, "allowed-namespaces", allowedEnv, "comma-separated list of allowed namespaces")

	fs.BoolVar(&cfg.S3.Enabled, "enable-s3-provisioning", cfg.S3.Enabled, "enable S3 provisioning")
	fs.StringVar(&cfg.S3.StorageClass, "s3-storage-class", cfg.S3.StorageClass, "storage class used for ObjectBucketClaim")
	fs.StringVar(&cfg.S3.SecretNameTemplate, "s3-secret-name-template", cfg.S3.SecretNameTemplate, "go template for canonical S3 secret name ({{name}} = instance)")
	fs.StringVar(&cfg.S3.EndpointScheme, "s3-endpoint-scheme", cfg.S3.EndpointScheme, "scheme to use when constructing S3 endpoint (http or https)")
	fs.DurationVar(&cfg.S3.ProvisionTimeout, "s3-provision-timeout", cfg.S3.ProvisionTimeout, "timeout waiting for S3 bucket/OBC binding")

	secretKeysDefault := s3SecretKeysRaw(cfg.S3.SecretKeys)
	if s3SecretKeysEnv != "" {
		secretKeysDefault = s3SecretKeysEnv
	}
	var s3SecretKeysFlag string
	fs.StringVar(&s3SecretKeysFlag, "s3-secret-keys", secretKeysDefault, "comma-separated S3 secret key mapping (endpoint=...,bucket=...,region=...,accessKeyId=...,secretAccessKey=...)")

	fs.BoolVar(&cfg.CNPG.Enabled, "enable-db-provisioning", cfg.CNPG.Enabled, "enable Postgres provisioning")
	fs.StringVar(&cfg.CNPG.Host, "cnpg-host", cfg.CNPG.Host, "CNPG host")
	fs.IntVar(&cfg.CNPG.Port, "cnpg-port", cfg.CNPG.Port, "CNPG port")
	fs.StringVar(&cfg.CNPG.AdminSecret, "cnpg-admin-secret", cfg.CNPG.AdminSecret, "CNPG admin secret name")
	fs.StringVar(&cfg.CNPG.JobNamespace, "cnpg-job-namespace", cfg.CNPG.JobNamespace, "namespace for CNPG provisioning Jobs")
	fs.StringVar(&cfg.CNPG.JobImage, "cnpg-job-image", cfg.CNPG.JobImage, "container image for CNPG Jobs")
	fs.StringVar(&cfg.CNPG.SSLMode, "cnpg-sslmode", cfg.CNPG.SSLMode, "SSL mode for Postgres connection (require/verify-ca/verify-full/disable)")
	fs.StringVar(&cfg.CNPG.DBSecretNameTemplate, "db-secret-name-template", cfg.CNPG.DBSecretNameTemplate, "go template for canonical DB secret name ({{name}} = instance)")
	fs.DurationVar(&cfg.CNPG.JobTimeout, "cnpg-job-timeout", cfg.CNPG.JobTimeout, "timeout waiting for CNPG Jobs to complete")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	cfg.Mode = strings.ToLower(cfg.Mode)
	cfg.AllowedNamespaces = splitAndClean(allowed)

	parsedKeys, err := parseS3SecretKeys(s3SecretKeysFlag, cfg.S3.SecretKeys)
	if err != nil {
		return Config{}, err
	}
	cfg.S3.SecretKeys = parsedKeys

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
	if c.S3.Enabled {
		if c.S3.StorageClass == "" {
			return fmt.Errorf("s3 storage class is required when S3 provisioning is enabled")
		}
		if c.S3.SecretNameTemplate == "" {
			return fmt.Errorf("s3 secret name template is required when S3 provisioning is enabled")
		}
		if c.S3.EndpointScheme == "" {
			return fmt.Errorf("s3 endpoint scheme is required when S3 provisioning is enabled")
		}
		if c.S3.ProvisionTimeout <= 0 {
			return fmt.Errorf("s3 provision timeout must be positive")
		}
		if c.S3.SecretKeys.Endpoint == "" || c.S3.SecretKeys.Bucket == "" || c.S3.SecretKeys.AccessKeyID == "" || c.S3.SecretKeys.SecretAccessKey == "" {
			return fmt.Errorf("s3 secret key names must be provided when S3 provisioning is enabled")
		}
	}
	if c.CNPG.Enabled {
		if c.CNPG.Host == "" {
			return fmt.Errorf("cnpg host is required when DB provisioning is enabled")
		}
		if c.CNPG.Port <= 0 || c.CNPG.Port > 65535 {
			return fmt.Errorf("cnpg port must be between 1 and 65535")
		}
		if c.CNPG.AdminSecret == "" {
			return fmt.Errorf("cnpg admin secret is required when DB provisioning is enabled")
		}
		if c.CNPG.JobNamespace == "" {
			return fmt.Errorf("cnpg job namespace is required when DB provisioning is enabled")
		}
		if c.CNPG.JobImage == "" {
			return fmt.Errorf("cnpg job image is required when DB provisioning is enabled")
		}
		if c.CNPG.SSLMode == "" {
			return fmt.Errorf("cnpg sslmode is required when DB provisioning is enabled")
		}
		if c.CNPG.DBSecretNameTemplate == "" {
			return fmt.Errorf("db secret name template is required when DB provisioning is enabled")
		}
		if c.CNPG.JobTimeout <= 0 {
			return fmt.Errorf("cnpg job timeout must be positive")
		}
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

func parseS3SecretKeys(raw string, base S3SecretKeys) (S3SecretKeys, error) {
	if raw == "" {
		return base, nil
	}
	out := base
	pairs := strings.Split(raw, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			return S3SecretKeys{}, fmt.Errorf("invalid s3 secret key mapping %q", pair)
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		switch key {
		case "endpoint":
			out.Endpoint = val
		case "bucket":
			out.Bucket = val
		case "region":
			out.Region = val
		case "accessKeyId":
			out.AccessKeyID = val
		case "secretAccessKey":
			out.SecretAccessKey = val
		default:
			return S3SecretKeys{}, fmt.Errorf("unknown s3 secret key %q", key)
		}
	}
	return out, nil
}

func s3SecretKeysRaw(keys S3SecretKeys) string {
	parts := []string{
		"endpoint=" + keys.Endpoint,
		"bucket=" + keys.Bucket,
		"region=" + keys.Region,
		"accessKeyId=" + keys.AccessKeyID,
		"secretAccessKey=" + keys.SecretAccessKey,
	}
	return strings.Join(parts, ",")
}
