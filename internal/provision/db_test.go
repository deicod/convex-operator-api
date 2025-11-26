package provision

import (
	"context"
	"testing"
	"time"

	"github.com/deicod/convex-operator/api/v1alpha1"
	"github.com/deicod/convex-operator-api/internal/config"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCNPGProvisionerCreatesJobAndSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	mustAddJobScheme(t, scheme)

	adminSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pg-rw-superuser",
			Namespace: "postgres",
		},
		Data: map[string][]byte{
			"username": []byte("admin"),
			"password": []byte("adminpw"),
		},
	}

	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(adminSecret).Build()
	prov := CNPGProvisioner{
		Client:       cli,
		Config:       defaultCNPGConfig(),
		PollInterval: 10 * time.Millisecond,
		PasswordGenerator: func() (string, error) {
			return "secretpw", nil
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	resultCh := make(chan struct {
		res DBResult
		err error
	}, 1)
	go func() {
		res, err := prov.Provision(ctx, "demo", "default")
		resultCh <- struct {
			res DBResult
			err error
		}{res: res, err: err}
	}()

	job := waitForJob(ctx, t, cli, "convex-demo-db-create", "postgres")
	markJobSucceeded(ctx, t, cli, job)

	out := <-resultCh
	if out.err != nil {
		t.Fatalf("expected success, got %v", out.err)
	}
	if out.res.SecretName != "convex-demo-db" {
		t.Fatalf("unexpected secret name: %s", out.res.SecretName)
	}

	var dbSecret corev1.Secret
	if err := cli.Get(ctx, types.NamespacedName{Name: out.res.SecretName, Namespace: "default"}, &dbSecret); err != nil {
		t.Fatalf("expected db secret, got error: %v", err)
	}
	assertSecretValue(t, dbSecret, dbSecretKeyURL, "postgres://demo:secretpw@pg-rw.postgres.svc.cluster.local:5432/demo?sslmode=require")
	assertSecretValue(t, dbSecret, dbSecretKeyUser, "demo")
	assertSecretValue(t, dbSecret, dbSecretKeyPassword, "secretpw")
	assertSecretValue(t, dbSecret, dbSecretKeyName, "demo")
	assertSecretValue(t, dbSecret, dbSecretKeyHost, "pg-rw.postgres.svc.cluster.local")
	assertSecretValue(t, dbSecret, dbSecretKeyPort, "5432")
	assertSecretValue(t, dbSecret, dbSecretKeySSLMode, "require")
}

func TestCNPGProvisionerTimeout(t *testing.T) {
	scheme := runtime.NewScheme()
	mustAddJobScheme(t, scheme)

	adminSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pg-rw-superuser",
			Namespace: "postgres",
		},
	}

	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(adminSecret).Build()
	cfg := defaultCNPGConfig()
	cfg.JobTimeout = 30 * time.Millisecond
	prov := CNPGProvisioner{
		Client:       cli,
		Config:       cfg,
		PollInterval: 10 * time.Millisecond,
		PasswordGenerator: func() (string, error) {
			return "secretpw", nil
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := prov.Provision(ctx, "demo", "default")
	if err == nil {
		t.Fatalf("expected timeout error")
	}

	// Job should be removed on failure.
	var job batchv1.Job
	if err := cli.Get(ctx, types.NamespacedName{Name: "convex-demo-db-create", Namespace: "postgres"}, &job); err == nil {
		t.Fatalf("expected job cleanup")
	} else if !apierrors.IsNotFound(err) {
		t.Fatalf("expected notfound error, got %v", err)
	}
}

func TestApplyDBSpec(t *testing.T) {
	spec := v1alpha1.ConvexInstanceSpec{
		Backend: v1alpha1.BackendSpec{
			DB: v1alpha1.BackendDatabaseSpec{},
		},
	}

	ApplyDBSpec(&spec, "db-secret")

	if spec.Backend.DB.Engine != "postgres" {
		t.Fatalf("expected postgres engine")
	}
	if spec.Backend.DB.SecretRef != "db-secret" {
		t.Fatalf("unexpected secret ref: %s", spec.Backend.DB.SecretRef)
	}
	if spec.Backend.DB.URLKey != dbSecretKeyURL {
		t.Fatalf("unexpected url key: %s", spec.Backend.DB.URLKey)
	}
}

func mustAddJobScheme(t *testing.T, scheme *runtime.Scheme) {
	t.Helper()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add corev1: %v", err)
	}
	if err := batchv1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add batchv1: %v", err)
	}
}

func defaultCNPGConfig() config.CNPGConfig {
	return config.CNPGConfig{
		Enabled:              true,
		Host:                 "pg-rw.postgres.svc.cluster.local",
		Port:                 5432,
		AdminSecret:          "pg-rw-superuser",
		JobNamespace:         "postgres",
		JobImage:             "postgres:18-alpine",
		SSLMode:              "require",
		DBSecretNameTemplate: "convex-{{name}}-db",
		JobTimeout:           2 * time.Second,
	}
}

func waitForJob(ctx context.Context, t *testing.T, c client.Client, name, namespace string) *batchv1.Job {
	t.Helper()
	var lastErr error
	for i := 0; i < 50; i++ {
		job := &batchv1.Job{}
		err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, job)
		if err == nil {
			return job
		}
		lastErr = err
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("job %s/%s not created: %v", namespace, name, lastErr)
	return nil
}

func markJobSucceeded(ctx context.Context, t *testing.T, c client.Client, job *batchv1.Job) {
	t.Helper()
	job.Status.Succeeded = 1
	if err := c.Status().Update(ctx, job); err != nil {
		// fallback for fake client without status subresource
		if err := c.Update(ctx, job); err != nil {
			t.Fatalf("failed to mark job succeeded: %v", err)
		}
	}
	if err := c.Get(ctx, types.NamespacedName{Name: job.Name, Namespace: job.Namespace}, job); err != nil {
		t.Fatalf("failed to mark job succeeded: %v", err)
	}
}
