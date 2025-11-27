package provision

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/deicod/convex-operator-api/internal/config"
	"github.com/deicod/convex-operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ErrDBProvisioningDisabled signals that DB provisioning is disabled by configuration.
var ErrDBProvisioningDisabled = errors.New("db provisioning disabled")

const (
	dbSecretKeyURL      = "url"
	dbSecretKeyUser     = "username"
	dbSecretKeyPassword = "password"
	dbSecretKeyName     = "database"
	dbSecretKeyHost     = "host"
	dbSecretKeyPort     = "port"
	dbSecretKeySSLMode  = "sslmode"
)

// DBResult describes the canonical DB secret produced by provisioning.
type DBResult struct {
	SecretName string
}

// CNPGProvisioner provisions Postgres user/db via a Job and writes a canonical secret.
type CNPGProvisioner struct {
	Client            client.Client
	Config            config.CNPGConfig
	PollInterval      time.Duration
	PasswordGenerator func() (string, error)
}

// Provision creates the DB user/db and canonical secret.
func (p CNPGProvisioner) Provision(ctx context.Context, instanceName, namespace string) (DBResult, error) {
	if !p.Config.Enabled {
		return DBResult{}, ErrDBProvisioningDisabled
	}
	if instanceName == "" || namespace == "" {
		return DBResult{}, fmt.Errorf("instance name and namespace are required")
	}

	user := sanitizeDBIdentifier(instanceName)
	dbName := user

	canonicalSecretName := renderNameTemplate(p.Config.DBSecretNameTemplate, instanceName)
	password := p.readExistingPassword(ctx, canonicalSecretName, namespace)
	if password == "" {
		if p.PasswordGenerator != nil {
			pw, err := p.PasswordGenerator()
			if err != nil {
				return DBResult{}, fmt.Errorf("generate password: %w", err)
			}
			password = pw
		} else {
			password = randomPassword(32)
		}
	}

	jobName := fmt.Sprintf("convex-%s-db-create", user)
	createJob := buildCNPGJob(jobName, p.Config, user, dbName, password)
	jobKey := types.NamespacedName{Name: jobName, Namespace: p.Config.JobNamespace}

	existingJob := &batchv1.Job{}
	jobExists := false
	if err := p.Client.Get(ctx, jobKey, existingJob); err != nil {
		if !apierrors.IsNotFound(err) {
			return DBResult{}, err
		}
	} else {
		jobExists = true
		if jobPw := extractPasswordFromJob(existingJob); jobPw != "" {
			password = jobPw
		} else if password == "" {
			return DBResult{}, fmt.Errorf("existing job %s/%s present but password is unknown; delete job or secret and retry", jobKey.Namespace, jobKey.Name)
		}
	}

	if !jobExists {
		if err := p.Client.Create(ctx, createJob); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return DBResult{}, err
			}
		}
	}

	if err := p.waitForJobCompletion(ctx, jobKey); err != nil {
		p.cleanupJob(jobName)
		return DBResult{}, err
	}

	if err := p.writeDBSecret(ctx, canonicalSecretName, namespace, user, password, dbName); err != nil {
		return DBResult{}, err
	}

	return DBResult{SecretName: canonicalSecretName}, nil
}

// Cleanup launches best-effort cleanup (drop DB/user) and deletes the canonical secret.
func (p CNPGProvisioner) Cleanup(ctx context.Context, instanceName, namespace string) {
	secretName := renderNameTemplate(p.Config.DBSecretNameTemplate, instanceName)
	_ = p.deleteSecret(ctx, namespace, secretName)

	user := sanitizeDBIdentifier(instanceName)
	jobName := fmt.Sprintf("convex-%s-db-drop", user)
	dropJob := buildCNPGDropJob(jobName, p.Config, user)
	_ = p.Client.Create(context.Background(), dropJob) // best-effort fire-and-forget
}

func (p CNPGProvisioner) writeDBSecret(ctx context.Context, name, namespace, user, password, dbName string) error {
	host := p.Config.Host
	port := p.Config.Port
	sslmode := p.Config.SSLMode

	url := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s", user, password, host, port, dbName, sslmode)

	data := map[string][]byte{
		dbSecretKeyURL:      []byte(url),
		dbSecretKeyUser:     []byte(user),
		dbSecretKeyPassword: []byte(password),
		dbSecretKeyName:     []byte(dbName),
		dbSecretKeyHost:     []byte(host),
		dbSecretKeyPort:     []byte(fmt.Sprintf("%d", port)),
		dbSecretKeySSLMode:  []byte(sslmode),
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}

	if err := p.Client.Create(ctx, secret); err != nil {
		if apierrors.IsAlreadyExists(err) {
			existing := &corev1.Secret{}
			if err := p.Client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, existing); err != nil {
				return err
			}
			existing.Type = corev1.SecretTypeOpaque
			existing.Data = data
			return p.Client.Update(ctx, existing)
		}
		return err
	}
	return nil
}

func (p CNPGProvisioner) readExistingPassword(ctx context.Context, secretName, namespace string) string {
	secret := &corev1.Secret{}
	if err := p.Client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, secret); err != nil {
		return ""
	}
	pw := secret.Data[dbSecretKeyPassword]
	return string(pw)
}

func (p CNPGProvisioner) deleteSecret(ctx context.Context, namespace, name string) error {
	return client.IgnoreNotFound(p.Client.Delete(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}))
}

func (p CNPGProvisioner) waitForJobCompletion(ctx context.Context, key types.NamespacedName) error {
	timeout := p.Config.JobTimeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	interval := p.pollInterval()

	var job batchv1.Job
	return wait.PollUntilContextTimeout(ctx, interval, timeout, true, func(ctx context.Context) (bool, error) {
		if err := p.Client.Get(ctx, key, &job); err != nil {
			return false, err
		}
		if job.Status.Failed > 0 {
			return false, fmt.Errorf("job %s/%s failed", key.Namespace, key.Name)
		}
		if job.Status.Succeeded > 0 {
			return true, nil
		}
		return false, nil
	})
}

func (p CNPGProvisioner) cleanupJob(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = p.Client.Delete(ctx, &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: p.Config.JobNamespace,
		},
	})
}

func (p CNPGProvisioner) pollInterval() time.Duration {
	if p.PollInterval > 0 {
		return p.PollInterval
	}
	return time.Second
}

func buildCNPGJob(name string, cfg config.CNPGConfig, user, dbName, password string) *batchv1.Job {
	backoff := int32(0)
	ttl := int32(300)
	quotedUser := quoteIdent(user)
	quotedDB := quoteIdent(dbName)
	quotedPassword := quoteLiteral(password)
	sql := fmt.Sprintf(`DO $body$
BEGIN
	IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = %s) THEN
		CREATE ROLE %s LOGIN PASSWORD %s;
	ELSE
		ALTER ROLE %s LOGIN PASSWORD %s;
	END IF;

	IF NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = %s) THEN
		CREATE DATABASE %s OWNER %s;
	END IF;

	GRANT ALL PRIVILEGES ON DATABASE %s TO %s;
END
$body$;`, quoteLiteral(user), quotedUser, quotedPassword, quotedUser, quotedPassword, quoteLiteral(dbName), quotedDB, quotedUser, quotedDB, quotedUser)
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cfg.JobNamespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoff,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: cfg.JobServiceAccount,
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:  "init-db",
							Image: cfg.JobImage,
							Command: []string{
								"sh", "-c",
								fmt.Sprintf("psql \"host=%s port=%d user=$ADMIN_USER password=$ADMIN_PASSWORD sslmode=%s dbname=postgres\" -v ON_ERROR_STOP=1 <<'SQL'\n%s\nSQL\n", cfg.Host, cfg.Port, cfg.SSLMode, sql),
							},
							Env: []corev1.EnvVar{
								{
									Name: "ADMIN_USER",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: cfg.AdminSecret},
											Key:                  "username",
										},
									},
								},
								{
									Name: "ADMIN_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: cfg.AdminSecret},
											Key:                  "password",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func buildCNPGDropJob(name string, cfg config.CNPGConfig, user string) *batchv1.Job {
	backoff := int32(0)
	ttl := int32(300)
	sql := fmt.Sprintf(`DO $body$
BEGIN
	PERFORM pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = %s;
	EXECUTE format('DROP DATABASE IF EXISTS %%I', %s);
	EXECUTE format('DROP ROLE IF EXISTS %%I', %s);
END
$body$;`, quoteLiteral(user), quoteLiteral(user), quoteLiteral(user))

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cfg.JobNamespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoff,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: cfg.JobServiceAccount,
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:  "drop-db",
							Image: cfg.JobImage,
							Command: []string{
								"sh", "-c",
								fmt.Sprintf("psql \"host=%s port=%d user=$ADMIN_USER password=$ADMIN_PASSWORD sslmode=%s dbname=postgres\" -v ON_ERROR_STOP=1 <<'SQL'\n%s\nSQL\n", cfg.Host, cfg.Port, cfg.SSLMode, sql),
							},
							Env: []corev1.EnvVar{
								{
									Name: "ADMIN_USER",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: cfg.AdminSecret},
											Key:                  "username",
										},
									},
								},
								{
									Name: "ADMIN_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: cfg.AdminSecret},
											Key:                  "password",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// ApplyDBSpec mutates a ConvexInstanceSpec to reference the canonical DB secret.
func ApplyDBSpec(spec *v1alpha1.ConvexInstanceSpec, secretName string) {
	if spec == nil {
		return
	}
	spec.Backend.DB.Engine = "postgres"
	spec.Backend.DB.SecretRef = secretName
	spec.Backend.DB.URLKey = dbSecretKeyURL
}

var dbIdentAllowed = regexp.MustCompile(`[^a-z0-9\-]`)

func sanitizeDBIdentifier(name string) string {
	lower := strings.ToLower(name)
	safe := dbIdentAllowed.ReplaceAllString(lower, "-")
	safe = strings.Trim(safe, "-")
	if safe == "" {
		return "convex"
	}
	if len(safe) > 63 {
		safe = safe[:63]
	}
	return safe
}

func randomPassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			b[i] = charset[i%len(charset)]
			continue
		}
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

func quoteIdent(ident string) string {
	escaped := strings.ReplaceAll(ident, `"`, `""`)
	return `"` + escaped + `"`
}

func quoteLiteral(lit string) string {
	escaped := strings.ReplaceAll(lit, `'`, `''`)
	return `'` + escaped + `'`
}

func extractPasswordFromJob(job *batchv1.Job) string {
	if job == nil {
		return ""
	}
	for _, c := range job.Spec.Template.Spec.Containers {
		cmd := strings.Join(c.Command, " ")
		if cmd == "" {
			continue
		}
		// look for PASSWORD 'value'
		re := regexp.MustCompile(`PASSWORD '([^']+)'`)
		matches := re.FindStringSubmatch(cmd)
		if len(matches) >= 2 {
			return matches[1]
		}
	}
	return ""
}
