package provision

import (
	"context"
	"testing"
	"time"

	"github.com/deicod/convex-operator-api/internal/config"
	"github.com/deicod/convex-operator/api/v1alpha1"
	objectbucketv1alpha1 "github.com/kube-object-storage/lib-bucket-provisioner/pkg/apis/objectbucket.io/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestS3ProvisionerCreatesAndBinds(t *testing.T) {
	scheme := runtime.NewScheme()
	mustAddToScheme(t, scheme)

	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	prov := S3Provisioner{
		Client:       cli,
		Config:       defaultConfig(),
		PollInterval: 10 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	resultCh := make(chan struct {
		res S3Result
		err error
	}, 1)
	go func() {
		res, err := prov.Provision(ctx, "demo", "default")
		resultCh <- struct {
			res S3Result
			err error
		}{res: res, err: err}
	}()

	waitForOBC(ctx, t, cli, "demo", "default")
	bindOBCAndSecret(ctx, t, cli, "demo", "default", "s3.example.com", "443", "eu-central-1")

	out := <-resultCh
	if out.err != nil {
		t.Fatalf("expected success, got %v", out.err)
	}
	if out.res.SecretName != "convex-demo-s3" {
		t.Fatalf("unexpected secret name: %s", out.res.SecretName)
	}

	var canonical corev1.Secret
	if err := cli.Get(ctx, types.NamespacedName{Name: out.res.SecretName, Namespace: "default"}, &canonical); err != nil {
		t.Fatalf("expected canonical secret, got error: %v", err)
	}
	assertSecretValue(t, canonical, prov.Config.SecretKeys.Endpoint, "https://s3.example.com:443")
	assertSecretValue(t, canonical, prov.Config.SecretKeys.Bucket, "demo")
	assertSecretValue(t, canonical, prov.Config.SecretKeys.Region, "eu-central-1")
	assertSecretValue(t, canonical, prov.Config.SecretKeys.AccessKeyID, "access")
	assertSecretValue(t, canonical, prov.Config.SecretKeys.SecretAccessKey, "secret")
}

func TestS3ProvisionerTimeout(t *testing.T) {
	scheme := runtime.NewScheme()
	mustAddToScheme(t, scheme)

	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	cfg := defaultConfig()
	cfg.ProvisionTimeout = 50 * time.Millisecond
	prov := S3Provisioner{
		Client:       cli,
		Config:       cfg,
		PollInterval: 10 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := prov.Provision(ctx, "demo", "default")
	if err == nil {
		t.Fatalf("expected timeout error")
	}

	// Expect cleanup of created OBC.
	var obc objectbucketv1alpha1.ObjectBucketClaim
	if err := cli.Get(ctx, types.NamespacedName{Name: "demo", Namespace: "default"}, &obc); err == nil {
		t.Fatalf("expected obc to be cleaned up on failure")
	} else if !apierrors.IsNotFound(err) {
		t.Fatalf("expected notfound error, got %v", err)
	}
}

func TestS3ProvisionerExistingBoundClaim(t *testing.T) {
	scheme := runtime.NewScheme()
	mustAddToScheme(t, scheme)

	obc := &objectbucketv1alpha1.ObjectBucketClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "default",
		},
		Spec: objectbucketv1alpha1.ObjectBucketClaimSpec{
			StorageClassName: "rook-ceph-bucket",
			BucketName:       "demo",
		},
		Status: objectbucketv1alpha1.ObjectBucketClaimStatus{
			Phase: objectbucketv1alpha1.ObjectBucketClaimStatusPhaseBound,
		},
	}
	sourceSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"BUCKET_HOST":                       []byte("s3.example.com"),
			"BUCKET_NAME":                       []byte("demo"),
			"BUCKET_REGION":                     []byte("eu-central-1"),
			objectbucketv1alpha1.AwsKeyField:    []byte("access"),
			objectbucketv1alpha1.AwsSecretField: []byte("secret"),
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obc, sourceSecret).Build()
	prov := S3Provisioner{
		Client:       cli,
		Config:       defaultConfig(),
		PollInterval: 10 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	res, err := prov.Provision(ctx, "demo", "default")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if res.SecretName != "convex-demo-s3" {
		t.Fatalf("unexpected secret name: %s", res.SecretName)
	}
}

func TestS3ProvisionerCleansUpOnSecretError(t *testing.T) {
	scheme := runtime.NewScheme()
	mustAddToScheme(t, scheme)

	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	cfg := defaultConfig()
	cfg.ProvisionTimeout = 200 * time.Millisecond
	prov := S3Provisioner{
		Client:       cli,
		Config:       cfg,
		PollInterval: 10 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	resultCh := make(chan error, 1)
	go func() {
		_, err := prov.Provision(ctx, "demo", "default")
		resultCh <- err
	}()

	waitForOBC(ctx, t, cli, "demo", "default")
	// Bind OBC but leave BUCKET_HOST missing to trigger error.
	var obc objectbucketv1alpha1.ObjectBucketClaim
	if err := cli.Get(ctx, types.NamespacedName{Name: "demo", Namespace: "default"}, &obc); err != nil {
		t.Fatalf("failed to get obc: %v", err)
	}
	obc.Status.Phase = objectbucketv1alpha1.ObjectBucketClaimStatusPhaseBound
	if err := cli.Update(ctx, &obc); err != nil {
		t.Fatalf("failed to update obc status: %v", err)
	}
	sourceSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"BUCKET_NAME":                       []byte("demo"),
			objectbucketv1alpha1.AwsKeyField:    []byte("access"),
			objectbucketv1alpha1.AwsSecretField: []byte("secret"),
		},
	}
	if err := cli.Create(ctx, sourceSecret); err != nil {
		t.Fatalf("failed to create source secret: %v", err)
	}

	err := <-resultCh
	if err == nil {
		t.Fatalf("expected error due to missing host")
	}

	// OBC should be removed on failure.
	if err := cli.Get(ctx, types.NamespacedName{Name: "demo", Namespace: "default"}, &obc); err == nil {
		t.Fatalf("expected obc cleanup after failure")
	} else if !apierrors.IsNotFound(err) {
		t.Fatalf("expected notfound error, got %v", err)
	}
}

func TestApplyS3Spec(t *testing.T) {
	spec := fakeSpec()
	ApplyS3Spec(&spec, "secret-name", config.S3SecretKeys{
		Endpoint:        "endpoint",
		Bucket:          "bucket",
		AccessKeyID:     "ak",
		SecretAccessKey: "sk",
	})

	if !spec.Backend.S3.Enabled {
		t.Fatalf("expected s3 enabled")
	}
	if spec.Backend.S3.SecretRef != "secret-name" {
		t.Fatalf("unexpected secret ref: %s", spec.Backend.S3.SecretRef)
	}
	if spec.Backend.S3.EndpointKey != "endpoint" || spec.Backend.S3.BucketKey != "bucket" || spec.Backend.S3.AccessKeyIDKey != "ak" || spec.Backend.S3.SecretAccessKeyKey != "sk" {
		t.Fatalf("unexpected key mapping: %+v", spec.Backend.S3)
	}
}

func mustAddToScheme(t *testing.T, scheme *runtime.Scheme) {
	t.Helper()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add corev1 to scheme: %v", err)
	}
	if err := objectbucketv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add obc scheme: %v", err)
	}
}

func defaultConfig() config.S3Config {
	return config.S3Config{
		Enabled:            true,
		StorageClass:       "rook-ceph-bucket",
		SecretNameTemplate: "convex-{{name}}-s3",
		EndpointScheme:     "https",
		SecretKeys: config.S3SecretKeys{
			Endpoint:        "endpoint",
			Bucket:          "bucket",
			Region:          "region",
			AccessKeyID:     "accessKeyId",
			SecretAccessKey: "secretAccessKey",
		},
		ProvisionTimeout: 2 * time.Second,
	}
}

func waitForOBC(ctx context.Context, t *testing.T, c client.Client, name, namespace string) {
	t.Helper()
	var lastErr error
	for i := 0; i < 50; i++ {
		obc := &objectbucketv1alpha1.ObjectBucketClaim{}
		err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, obc)
		if err == nil {
			return
		}
		lastErr = err
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("obc %s/%s not created: %v", namespace, name, lastErr)
}

func bindOBCAndSecret(ctx context.Context, t *testing.T, c client.Client, name, namespace, host, port, region string) {
	t.Helper()
	obc := &objectbucketv1alpha1.ObjectBucketClaim{}
	if err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, obc); err != nil {
		t.Fatalf("failed to get obc: %v", err)
	}
	obc.Status.Phase = objectbucketv1alpha1.ObjectBucketClaimStatusPhaseBound
	if err := c.Update(ctx, obc); err != nil {
		t.Fatalf("failed to update obc: %v", err)
	}

	sourceSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"BUCKET_HOST":                       []byte(host),
			"BUCKET_PORT":                       []byte(port),
			"BUCKET_NAME":                       []byte(name),
			"BUCKET_REGION":                     []byte(region),
			objectbucketv1alpha1.AwsKeyField:    []byte("access"),
			objectbucketv1alpha1.AwsSecretField: []byte("secret"),
		},
	}
	if err := c.Create(ctx, sourceSecret); err != nil {
		t.Fatalf("failed to create source secret: %v", err)
	}
}

func assertSecretValue(t *testing.T, secret corev1.Secret, key, want string) {
	t.Helper()
	got := string(secret.Data[key])
	if got != want {
		t.Fatalf("expected secret[%s]=%s, got %s", key, want, got)
	}
}

func fakeSpec() v1alpha1.ConvexInstanceSpec {
	return v1alpha1.ConvexInstanceSpec{
		Backend: v1alpha1.BackendSpec{
			S3: v1alpha1.BackendS3Spec{},
		},
	}
}
