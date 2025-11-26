package provision

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/deicod/convex-operator-api/internal/config"
	"github.com/deicod/convex-operator/api/v1alpha1"
	objectbucketv1alpha1 "github.com/kube-object-storage/lib-bucket-provisioner/pkg/apis/objectbucket.io/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ErrS3ProvisioningDisabled signals that S3 provisioning is disabled by configuration.
var ErrS3ProvisioningDisabled = errors.New("s3 provisioning disabled")

// S3Result describes the canonical secret produced by provisioning.
type S3Result struct {
	SecretName string
	SecretKeys config.S3SecretKeys
}

// S3Provisioner creates ObjectBucketClaims and canonical secrets for Convex instances.
type S3Provisioner struct {
	Client       client.Client
	Config       config.S3Config
	PollInterval time.Duration
}

// Provision ensures an ObjectBucketClaim exists and is Bound, then writes a canonical secret.
func (p S3Provisioner) Provision(ctx context.Context, instanceName, namespace string) (S3Result, error) {
	if !p.Config.Enabled {
		return S3Result{}, ErrS3ProvisioningDisabled
	}
	if instanceName == "" || namespace == "" {
		return S3Result{}, fmt.Errorf("instance name and namespace are required")
	}

	obcName := instanceName
	key := types.NamespacedName{Name: obcName, Namespace: namespace}

	created := false
	obc := &objectbucketv1alpha1.ObjectBucketClaim{}
	if err := p.Client.Get(ctx, key, obc); err != nil {
		if !apierrors.IsNotFound(err) {
			return S3Result{}, err
		}
		obc = &objectbucketv1alpha1.ObjectBucketClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      obcName,
				Namespace: namespace,
			},
			Spec: objectbucketv1alpha1.ObjectBucketClaimSpec{
				StorageClassName: p.Config.StorageClass,
				BucketName:       instanceName,
			},
		}
		if err := p.Client.Create(ctx, obc); err != nil {
			return S3Result{}, err
		}
		created = true
	} else if obc.Spec.StorageClassName != "" && obc.Spec.StorageClassName != p.Config.StorageClass {
		return S3Result{}, fmt.Errorf("existing objectbucketclaim %s/%s uses storageClass %q (expected %q)", namespace, obcName, obc.Spec.StorageClassName, p.Config.StorageClass)
	}

	boundOBC, err := p.waitForBound(ctx, key)
	if err != nil {
		p.cleanupCreatedOBC(created, obcName, namespace)
		return S3Result{}, err
	}

	sourceSecretName := boundOBC.Name
	sourceSecret := &corev1.Secret{}
	if err := p.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: sourceSecretName}, sourceSecret); err != nil {
		p.cleanupCreatedOBC(created, obcName, namespace)
		return S3Result{}, fmt.Errorf("fetching object bucket secret: %w", err)
	}

	endpoint, bucket, region, accessKeyID, secretAccessKey, err := extractBucketSecret(sourceSecret)
	if err != nil {
		p.cleanupCreatedOBC(created, obcName, namespace)
		return S3Result{}, err
	}

	canonicalEndpoint := buildEndpoint(p.Config.EndpointScheme, endpoint.Host, endpoint.Port)
	if canonicalEndpoint == "" {
		p.cleanupCreatedOBC(created, obcName, namespace)
		return S3Result{}, fmt.Errorf("bucket endpoint host is empty in secret %s/%s", namespace, sourceSecretName)
	}

	canonicalSecretName := renderNameTemplate(p.Config.SecretNameTemplate, instanceName)
	if err := p.writeCanonicalSecret(ctx, canonicalSecretName, namespace, canonicalEndpoint, bucket, region, accessKeyID, secretAccessKey); err != nil {
		p.cleanupCreatedOBC(created, obcName, namespace)
		return S3Result{}, err
	}

	return S3Result{
		SecretName: canonicalSecretName,
		SecretKeys: p.Config.SecretKeys,
	}, nil
}

func (p S3Provisioner) waitForBound(ctx context.Context, key types.NamespacedName) (*objectbucketv1alpha1.ObjectBucketClaim, error) {
	timeout := p.Config.ProvisionTimeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	interval := p.pollInterval()

	var latest objectbucketv1alpha1.ObjectBucketClaim
	err := wait.PollUntilContextTimeout(ctx, interval, timeout, true, func(ctx context.Context) (bool, error) {
		if err := p.Client.Get(ctx, key, &latest); err != nil {
			return false, err
		}
		switch latest.Status.Phase {
		case objectbucketv1alpha1.ObjectBucketClaimStatusPhaseBound:
			return true, nil
		case objectbucketv1alpha1.ObjectBucketClaimStatusPhaseFailed:
			return false, fmt.Errorf("objectbucketclaim %s/%s failed provisioning", key.Namespace, key.Name)
		default:
			return false, nil
		}
	})
	if err != nil {
		return nil, err
	}
	return &latest, nil
}

func (p S3Provisioner) writeCanonicalSecret(ctx context.Context, name, namespace, endpoint, bucket, region, accessKeyID, secretAccessKey string) error {
	data := map[string][]byte{
		p.Config.SecretKeys.Endpoint:        []byte(endpoint),
		p.Config.SecretKeys.Bucket:          []byte(bucket),
		p.Config.SecretKeys.AccessKeyID:     []byte(accessKeyID),
		p.Config.SecretKeys.SecretAccessKey: []byte(secretAccessKey),
	}
	if region != "" {
		data[p.Config.SecretKeys.Region] = []byte(region)
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

func (p S3Provisioner) cleanupCreatedOBC(created bool, name, namespace string) {
	if !created {
		return
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = p.Client.Delete(cleanupCtx, &objectbucketv1alpha1.ObjectBucketClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	})
}

func (p S3Provisioner) pollInterval() time.Duration {
	if p.PollInterval > 0 {
		return p.PollInterval
	}
	return time.Second
}

type bucketEndpoint struct {
	Host string
	Port string
}

func extractBucketSecret(secret *corev1.Secret) (bucketEndpoint, string, string, string, string, error) {
	host := string(secret.Data["BUCKET_HOST"])
	port := string(secret.Data["BUCKET_PORT"])
	bucket := string(secret.Data["BUCKET_NAME"])
	region := string(secret.Data["BUCKET_REGION"])
	accessKeyID := string(secret.Data[objectbucketv1alpha1.AwsKeyField])
	secretAccessKey := string(secret.Data[objectbucketv1alpha1.AwsSecretField])

	if host == "" {
		return bucketEndpoint{}, "", "", "", "", fmt.Errorf("bucket secret %s/%s missing BUCKET_HOST", secret.Namespace, secret.Name)
	}
	if bucket == "" {
		return bucketEndpoint{}, "", "", "", "", fmt.Errorf("bucket secret %s/%s missing BUCKET_NAME", secret.Namespace, secret.Name)
	}
	if accessKeyID == "" || secretAccessKey == "" {
		return bucketEndpoint{}, "", "", "", "", fmt.Errorf("bucket secret %s/%s missing access keys", secret.Namespace, secret.Name)
	}

	return bucketEndpoint{Host: host, Port: port}, bucket, region, accessKeyID, secretAccessKey, nil
}

func buildEndpoint(scheme, host, port string) string {
	if host == "" {
		return ""
	}
	if port == "" {
		return fmt.Sprintf("%s://%s", scheme, host)
	}
	return fmt.Sprintf("%s://%s:%s", scheme, host, port)
}

func renderNameTemplate(tmpl, name string) string {
	return strings.ReplaceAll(tmpl, "{{name}}", name)
}

// ApplyS3Spec mutates a ConvexInstanceSpec to reference the canonical S3 secret.
func ApplyS3Spec(spec *v1alpha1.ConvexInstanceSpec, secretName string, keys config.S3SecretKeys) {
	if spec == nil {
		return
	}
	spec.Backend.S3.Enabled = true
	spec.Backend.S3.SecretRef = secretName
	spec.Backend.S3.EndpointKey = keys.Endpoint
	spec.Backend.S3.AccessKeyIDKey = keys.AccessKeyID
	spec.Backend.S3.SecretAccessKeyKey = keys.SecretAccessKey
	spec.Backend.S3.BucketKey = keys.Bucket
}
