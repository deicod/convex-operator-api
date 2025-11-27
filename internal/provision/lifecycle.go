package provision

import (
	"context"
	"log"

	"github.com/deicod/convex-operator/api/v1alpha1"
)

// InstanceStore abstracts ConvexInstance CRUD operations.
type InstanceStore interface {
	ListConvexInstances(ctx context.Context, namespace string) ([]v1alpha1.ConvexInstance, error)
	GetConvexInstance(ctx context.Context, namespace, name string) (*v1alpha1.ConvexInstance, error)
	CreateConvexInstance(ctx context.Context, obj *v1alpha1.ConvexInstance) error
	UpdateConvexInstance(ctx context.Context, obj *v1alpha1.ConvexInstance) error
	DeleteConvexInstance(ctx context.Context, namespace, name string) error
}

type S3ProvisionerInterface interface {
	Provision(ctx context.Context, instanceName, namespace string) (S3Result, error)
	Cleanup(ctx context.Context, instanceName, namespace string)
}

type DBProvisionerInterface interface {
	Provision(ctx context.Context, instanceName, namespace string) (DBResult, error)
	Cleanup(ctx context.Context, instanceName, namespace string)
}

// LifecycleService wires provisioning into ConvexInstance CRUD.
type LifecycleService struct {
	Store InstanceStore
	S3    S3ProvisionerInterface
	DB    DBProvisionerInterface
	Log   *log.Logger
}

func (s LifecycleService) ListConvexInstances(ctx context.Context, namespace string) ([]v1alpha1.ConvexInstance, error) {
	return s.Store.ListConvexInstances(ctx, namespace)
}

func (s LifecycleService) GetConvexInstance(ctx context.Context, namespace, name string) (*v1alpha1.ConvexInstance, error) {
	return s.Store.GetConvexInstance(ctx, namespace, name)
}

func (s LifecycleService) CreateConvexInstance(ctx context.Context, obj *v1alpha1.ConvexInstance) error {
	s3Done := false
	dbDone := false

	if s.S3 != nil {
		res, err := s.S3.Provision(ctx, obj.Name, obj.Namespace)
		if err != nil {
			return err
		}
		ApplyS3Spec(&obj.Spec, res.SecretName, res.SecretKeys)
		s3Done = true
	}

	if s.DB != nil {
		res, err := s.DB.Provision(ctx, obj.Name, obj.Namespace)
		if err != nil {
			if s3Done {
				go s.S3.Cleanup(context.Background(), obj.Name, obj.Namespace)
			}
			return err
		}
		ApplyDBSpec(&obj.Spec, res.SecretName)
		dbDone = true
	}

	if err := s.Store.CreateConvexInstance(ctx, obj); err != nil {
		if s3Done {
			go s.S3.Cleanup(context.Background(), obj.Name, obj.Namespace)
		}
		if dbDone {
			go s.DB.Cleanup(context.Background(), obj.Name, obj.Namespace)
		}
		return err
	}

	return nil
}

func (s LifecycleService) UpdateConvexInstance(ctx context.Context, obj *v1alpha1.ConvexInstance) error {
	return s.Store.UpdateConvexInstance(ctx, obj)
}

func (s LifecycleService) DeleteConvexInstance(ctx context.Context, namespace, name string) error {
	if err := s.Store.DeleteConvexInstance(ctx, namespace, name); err != nil {
		return err
	}
	if s.S3 != nil {
		go s.S3.Cleanup(context.Background(), name, namespace)
	}
	if s.DB != nil {
		go s.DB.Cleanup(context.Background(), name, namespace)
	}
	return nil
}
