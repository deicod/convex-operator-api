package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/deicod/convex-operator/api/v1alpha1"
	jsonpatch "github.com/evanphx/json-patch/v5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InstanceRequest represents a create or update request body.
type InstanceRequest struct {
	Metadata ObjectMeta                  `json:"metadata"`
	Spec     v1alpha1.ConvexInstanceSpec `json:"spec"`
}

// ObjectMeta is a pared-down metadata view for requests/responses.
type ObjectMeta struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// InstanceResponse represents a response payload for a ConvexInstance.
type InstanceResponse struct {
	Metadata ObjectMeta                    `json:"metadata"`
	Spec     v1alpha1.ConvexInstanceSpec   `json:"spec"`
	Status   v1alpha1.ConvexInstanceStatus `json:"status,omitempty"`
}

// InstancesListResponse wraps a list response.
type InstancesListResponse struct {
	Items []InstanceResponse `json:"items"`
}

// Validate checks for required fields and known enums.
func (r InstanceRequest) Validate() error {
	if strings.TrimSpace(r.Metadata.Name) == "" {
		return errors.New("metadata.name is required")
	}
	if strings.TrimSpace(r.Metadata.Namespace) == "" {
		return errors.New("metadata.namespace is required")
	}
	if strings.TrimSpace(r.Spec.Environment) == "" {
		return errors.New("spec.environment is required")
	}
	if r.Spec.Environment != "dev" && r.Spec.Environment != "prod" {
		return fmt.Errorf("spec.environment must be one of: dev, prod")
	}
	if strings.TrimSpace(r.Spec.Version) == "" {
		return errors.New("spec.version is required")
	}
	if r.Spec.Maintenance.UpgradeStrategy != "" &&
		r.Spec.Maintenance.UpgradeStrategy != "inPlace" &&
		r.Spec.Maintenance.UpgradeStrategy != "exportImport" {
		return fmt.Errorf("spec.maintenance.upgradeStrategy must be one of: inPlace, exportImport")
	}
	return nil
}

// ToConvexInstance constructs a ConvexInstance from the request.
func (r InstanceRequest) ToConvexInstance() v1alpha1.ConvexInstance {
	return v1alpha1.ConvexInstance{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConvexInstance",
			APIVersion: "convex.icod.de/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.Metadata.Name,
			Namespace: r.Metadata.Namespace,
		},
		Spec: r.Spec,
	}
}

// FromConvexInstance builds a response DTO from the CR.
func FromConvexInstance(ci v1alpha1.ConvexInstance) InstanceResponse {
	return InstanceResponse{
		Metadata: ObjectMeta{
			Name:      ci.Name,
			Namespace: ci.Namespace,
		},
		Spec:   ci.Spec,
		Status: ci.Status,
	}
}

// ApplySpecPatch merges a JSON object patch onto a ConvexInstanceSpec.
func ApplySpecPatch(spec v1alpha1.ConvexInstanceSpec, patchBytes []byte) (v1alpha1.ConvexInstanceSpec, error) {
	origBytes, err := json.Marshal(spec)
	if err != nil {
		return v1alpha1.ConvexInstanceSpec{}, err
	}
	patched, err := mergePatch(origBytes, patchBytes)
	if err != nil {
		return v1alpha1.ConvexInstanceSpec{}, err
	}
	var out v1alpha1.ConvexInstanceSpec
	if err := json.Unmarshal(patched, &out); err != nil {
		return v1alpha1.ConvexInstanceSpec{}, err
	}
	return out, nil
}

// mergePatch is overrideable for tests.
var mergePatch = func(original, patch []byte) ([]byte, error) {
	return jsonpatch.MergePatch(original, patch)
}
