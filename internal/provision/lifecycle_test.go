package provision

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/deicod/convex-operator-api/internal/config"
	"github.com/deicod/convex-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type stubStore struct {
	createErr error
	deleteErr error
	created   *v1alpha1.ConvexInstance
	deleted   []string
}

func (s *stubStore) ListConvexInstances(_ context.Context, namespace string) ([]v1alpha1.ConvexInstance, error) {
	return nil, nil
}

func (s *stubStore) GetConvexInstance(_ context.Context, namespace, name string) (*v1alpha1.ConvexInstance, error) {
	return nil, nil
}

func (s *stubStore) CreateConvexInstance(_ context.Context, obj *v1alpha1.ConvexInstance) error {
	if s.createErr != nil {
		return s.createErr
	}
	s.created = obj
	return nil
}

func (s *stubStore) UpdateConvexInstance(_ context.Context, obj *v1alpha1.ConvexInstance) error {
	return nil
}

func (s *stubStore) DeleteConvexInstance(_ context.Context, namespace, name string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.deleted = append(s.deleted, namespace+"/"+name)
	return nil
}

type stubS3Provisioner struct {
	res        S3Result
	err        error
	cleanupCnt int
}

func (p *stubS3Provisioner) Provision(_ context.Context, instanceName, namespace string) (S3Result, error) {
	return p.res, p.err
}

func (p *stubS3Provisioner) Cleanup(_ context.Context, instanceName, namespace string) {
	p.cleanupCnt++
}

type stubDBProvisioner struct {
	res        DBResult
	err        error
	cleanupCnt int
}

func (p *stubDBProvisioner) Provision(_ context.Context, instanceName, namespace string) (DBResult, error) {
	return p.res, p.err
}

func (p *stubDBProvisioner) Cleanup(_ context.Context, instanceName, namespace string) {
	p.cleanupCnt++
}

func TestLifecycleServiceCreateProvisioningApplied(t *testing.T) {
	store := &stubStore{}
	s3 := &stubS3Provisioner{res: S3Result{SecretName: "s3-secret", SecretKeys: configS3Keys()}}
	db := &stubDBProvisioner{res: DBResult{SecretName: "db-secret"}}
	svc := LifecycleService{Store: store, S3: s3, DB: db}

	obj := &v1alpha1.ConvexInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
		Spec:       v1alpha1.ConvexInstanceSpec{Backend: v1alpha1.BackendSpec{}},
	}
	if err := svc.CreateConvexInstance(context.Background(), obj); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj.Spec.Backend.S3.SecretRef != "s3-secret" || obj.Spec.Backend.DB.SecretRef != "db-secret" {
		t.Fatalf("expected secret refs applied, got %+v", obj.Spec.Backend)
	}
	if store.created == nil {
		t.Fatalf("expected store create called")
	}
}

func TestLifecycleServiceCreateCleansOnFailure(t *testing.T) {
	store := &stubStore{createErr: errors.New("boom")}
	s3 := &stubS3Provisioner{res: S3Result{SecretName: "s3-secret", SecretKeys: configS3Keys()}}
	db := &stubDBProvisioner{res: DBResult{SecretName: "db-secret"}}
	svc := LifecycleService{Store: store, S3: s3, DB: db}

	obj := &v1alpha1.ConvexInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
		Spec:       v1alpha1.ConvexInstanceSpec{Backend: v1alpha1.BackendSpec{}},
	}
	if err := svc.CreateConvexInstance(context.Background(), obj); err == nil {
		t.Fatalf("expected error")
	}
	// allow async cleanup to run
	time.Sleep(10 * time.Millisecond)
	if s3.cleanupCnt == 0 || db.cleanupCnt == 0 {
		t.Fatalf("expected cleanup to be invoked")
	}
}

func TestLifecycleServiceDeleteTriggersCleanup(t *testing.T) {
	store := &stubStore{}
	s3 := &stubS3Provisioner{}
	db := &stubDBProvisioner{}
	svc := LifecycleService{Store: store, S3: s3, DB: db}

	if err := svc.DeleteConvexInstance(context.Background(), "ns", "name"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.deleted) != 1 || store.deleted[0] != "ns/name" {
		t.Fatalf("expected delete call")
	}
	time.Sleep(10 * time.Millisecond)
	if s3.cleanupCnt == 0 || db.cleanupCnt == 0 {
		t.Fatalf("expected cleanup invoked")
	}
}

func configS3Keys() config.S3SecretKeys {
	return config.S3SecretKeys{
		Endpoint:        "endpoint",
		Bucket:          "bucket",
		AccessKeyID:     "ak",
		SecretAccessKey: "sk",
	}
}
