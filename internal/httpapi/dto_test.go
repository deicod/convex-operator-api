package httpapi

import (
	"errors"
	"strings"
	"testing"

	"github.com/deicod/convex-operator/api/v1alpha1"
)

func TestInstanceRequestValidation(t *testing.T) {
	tests := []struct {
		name    string
		req     InstanceRequest
		wantErr bool
	}{
		{
			name: "valid",
			req: InstanceRequest{
				Metadata: ObjectMeta{Name: "demo", Namespace: "ns"},
				Spec: v1alpha1.ConvexInstanceSpec{
					Environment: "dev",
					Version:     "1.0.0",
					Backend: v1alpha1.BackendSpec{
						DB: v1alpha1.BackendDatabaseSpec{Engine: "postgres"},
					},
					Networking: v1alpha1.NetworkingSpec{Host: "example.com"},
					Maintenance: v1alpha1.MaintenanceSpec{
						UpgradeStrategy: "inPlace",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing metadata",
			req: InstanceRequest{
				Metadata: ObjectMeta{Name: "", Namespace: ""},
				Spec:     v1alpha1.ConvexInstanceSpec{},
			},
			wantErr: true,
		},
		{
			name: "invalid environment",
			req: InstanceRequest{
				Metadata: ObjectMeta{Name: "demo", Namespace: "ns"},
				Spec: v1alpha1.ConvexInstanceSpec{
					Environment: "stage",
					Version:     "1.0.0",
					Backend:     v1alpha1.BackendSpec{DB: v1alpha1.BackendDatabaseSpec{Engine: "postgres"}},
					Networking:  v1alpha1.NetworkingSpec{Host: "example.com"},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid upgrade strategy",
			req: InstanceRequest{
				Metadata: ObjectMeta{Name: "demo", Namespace: "ns"},
				Spec: v1alpha1.ConvexInstanceSpec{
					Environment: "dev",
					Version:     "1.0.0",
					Backend:     v1alpha1.BackendSpec{DB: v1alpha1.BackendDatabaseSpec{Engine: "postgres"}},
					Networking:  v1alpha1.NetworkingSpec{Host: "example.com"},
					Maintenance: v1alpha1.MaintenanceSpec{UpgradeStrategy: "bad"},
				},
			},
			wantErr: true,
		},
		{
			name: "missing version",
			req: InstanceRequest{
				Metadata: ObjectMeta{Name: "demo", Namespace: "ns"},
				Spec: v1alpha1.ConvexInstanceSpec{
					Environment: "dev",
					Version:     "",
					Backend:     v1alpha1.BackendSpec{DB: v1alpha1.BackendDatabaseSpec{Engine: "postgres"}},
					Networking:  v1alpha1.NetworkingSpec{Host: "example.com"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.wantErr && err == nil {
				t.Fatalf("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestToFromConvexInstance(t *testing.T) {
	req := InstanceRequest{
		Metadata: ObjectMeta{Name: "demo", Namespace: "ns"},
		Spec: v1alpha1.ConvexInstanceSpec{
			Environment: "dev",
			Version:     "1.0.0",
			Backend:     v1alpha1.BackendSpec{DB: v1alpha1.BackendDatabaseSpec{Engine: "postgres"}},
			Networking:  v1alpha1.NetworkingSpec{Host: "example.com"},
			Maintenance: v1alpha1.MaintenanceSpec{UpgradeStrategy: "inPlace"},
		},
	}

	ci := req.ToConvexInstance()
	if ci.Name != "demo" || ci.Namespace != "ns" {
		t.Fatalf("unexpected metadata: %+v", ci.ObjectMeta)
	}
	if ci.Spec.Environment != "dev" || ci.Spec.Version != "1.0.0" {
		t.Fatalf("unexpected spec: %+v", ci.Spec)
	}

	resp := FromConvexInstance(ci)
	if resp.Metadata.Name != "demo" || resp.Metadata.Namespace != "ns" {
		t.Fatalf("unexpected response metadata: %+v", resp.Metadata)
	}
}

func TestApplySpecPatch(t *testing.T) {
	orig := v1alpha1.ConvexInstanceSpec{
		Environment: "dev",
		Version:     "1.0.0",
		Backend:     v1alpha1.BackendSpec{DB: v1alpha1.BackendDatabaseSpec{Engine: "postgres"}},
		Networking:  v1alpha1.NetworkingSpec{Host: "example.com"},
		Maintenance: v1alpha1.MaintenanceSpec{UpgradeStrategy: "inPlace"},
	}
	patch := `{"version":"2.0.0","maintenance":{"upgradeStrategy":"exportImport"}}`

	out, err := ApplySpecPatch(orig, []byte(patch))
	if err != nil {
		t.Fatalf("patch failed: %v", err)
	}

	if out.Version != "2.0.0" {
		t.Fatalf("expected version updated, got %s", out.Version)
	}
	if out.Maintenance.UpgradeStrategy != "exportImport" {
		t.Fatalf("expected upgradeStrategy updated, got %s", out.Maintenance.UpgradeStrategy)
	}
}

func TestApplySpecPatchInvalidJSON(t *testing.T) {
	orig := v1alpha1.ConvexInstanceSpec{}
	_, err := ApplySpecPatch(orig, []byte("{invalid"))
	if err == nil {
		t.Fatalf("expected error for invalid json")
	}
}

func TestApplySpecPatchInvalidMerge(t *testing.T) {
	// override mergePatch to simulate error
	origMerge := mergePatch
	defer func() { mergePatch = origMerge }()
	mergePatch = func(_, _ []byte) ([]byte, error) {
		return nil, errors.New("boom")
	}

	orig := v1alpha1.ConvexInstanceSpec{}
	if _, err := ApplySpecPatch(orig, []byte("{}")); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected merge error to propagate")
	}
}
