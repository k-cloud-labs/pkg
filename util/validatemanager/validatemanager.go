package validatemanager

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"

	"github.com/k-cloud-labs/pkg/client/listers/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/util"
	"github.com/k-cloud-labs/pkg/util/cue"
	"github.com/k-cloud-labs/pkg/util/slice"
)

// ValidateManager managers validate policies for operation
type ValidateManager interface {
	// ApplyValidatePolicies validate the object if one or more matched validate policy exist.
	ApplyValidatePolicies(rawObj *unstructured.Unstructured, operation string) (*ValidateResult, error)
}

type validateManagerImpl struct {
	cvpLister v1alpha1.ClusterValidatePolicyLister
}

type ValidateResult struct {
	Reason string `json:"reason"`
	Valid  bool   `json:"valid"`
}

func NewValidateManager(cvpLister v1alpha1.ClusterValidatePolicyLister) ValidateManager {
	return &validateManagerImpl{
		cvpLister: cvpLister,
	}
}

func (m *validateManagerImpl) ApplyValidatePolicies(rawObj *unstructured.Unstructured, operation string) (*ValidateResult, error) {
	cvps, err := m.cvpLister.List(labels.Everything())
	if err != nil {
		klog.ErrorS(err, "Failed to list validate policies.")
		return nil, err
	}

	if len(cvps) == 0 {
		klog.V(2).InfoS("No validate policy.", "resource", klog.KObj(rawObj))
		return &ValidateResult{
			Valid: true,
		}, nil
	}

	for _, cvp := range cvps {
		if util.ResourceMatchSelectors(rawObj, cvp.Spec.ResourceSelectors...) {
			for _, rule := range cvp.Spec.ValidateRules {
				if len(rule.TargetOperations) == 0 || slice.Exists(rule.TargetOperations, operation) {
					result, err := executeCue(rawObj, rule.Cue)
					if err != nil {
						klog.ErrorS(err, "Failed to apply validate policy.", "validatepolicy", cvp.Name, "resource", klog.KObj(rawObj))
						return result, err
					}
					klog.V(2).InfoS("Applied validate policy.", "validatepolicy", cvp.Name, "resource", klog.KObj(rawObj))
				}
			}
		}
	}

	return &ValidateResult{
		Valid: true,
	}, nil
}

func executeCue(rawObj *unstructured.Unstructured, template string) (*ValidateResult, error) {
	result := ValidateResult{}
	if err := cue.CueDoAndReturn(template, util.ParameterName, rawObj, util.ValidateOutputName, &result); err != nil {
		return nil, err
	}

	return &result, nil
}
