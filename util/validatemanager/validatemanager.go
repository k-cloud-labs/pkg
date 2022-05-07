package validatemanager

import (
	admissionv1 "k8s.io/api/admission/v1"
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
	ApplyValidatePolicies(obj *unstructured.Unstructured, oldObj *unstructured.Unstructured, operation admissionv1.Operation) (*ValidateResult, error)
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

func (m *validateManagerImpl) ApplyValidatePolicies(rawObj *unstructured.Unstructured, oldObj *unstructured.Unstructured, operation admissionv1.Operation) (*ValidateResult, error) {
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
		if len(cvp.Spec.ResourceSelectors) == 0 || util.ResourceMatchSelectors(rawObj, cvp.Spec.ResourceSelectors...) {
			for _, rule := range cvp.Spec.ValidateRules {
				if len(rule.TargetOperations) == 0 || slice.Exists(rule.TargetOperations, operation) {
					if operation == admissionv1.Update {
						oldObj = nil
					}
					result, err := executeCue(rawObj, oldObj, rule.Cue)
					if err != nil {
						klog.ErrorS(err, "Failed to apply validate policy.", "validatepolicy", cvp.Name, "resource", klog.KObj(rawObj))
						return nil, err
					}
					klog.V(2).InfoS("Applied validate policy.", "validatepolicy", cvp.Name, "resource", klog.KObj(rawObj))
					if !result.Valid {
						return result, nil
					}
				}
			}
		}
	}

	return &ValidateResult{
		Valid: true,
	}, nil
}

func executeCue(rawObj *unstructured.Unstructured, oldObj *unstructured.Unstructured, template string) (*ValidateResult, error) {
	result := ValidateResult{}
	parameters := []cue.Parameter{
		{
			Name:   util.ObjectParameterName,
			Object: rawObj,
		},
	}

	if oldObj != nil {
		parameters = append(parameters, cue.Parameter{
			Name:   util.OldObjectParameterName,
			Object: oldObj,
		})
	}
	if err := cue.CueDoAndReturn(template, parameters, util.ValidateOutputName, &result); err != nil {
		return nil, err
	}

	return &result, nil
}
