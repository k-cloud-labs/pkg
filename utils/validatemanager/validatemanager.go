package validatemanager

import (
	"fmt"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/client/listers/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/utils"
	"github.com/k-cloud-labs/pkg/utils/cue"
	"github.com/k-cloud-labs/pkg/utils/dynamicclient"
	"github.com/k-cloud-labs/pkg/utils/util"
)

// ValidateManager managers validate policies for operation
type ValidateManager interface {
	// ApplyValidatePolicies validate the object if one or more matched validate policy exist.
	ApplyValidatePolicies(obj *unstructured.Unstructured, oldObj *unstructured.Unstructured, operation admissionv1.Operation) (*ValidateResult, error)
}

type validateManagerImpl struct {
	dynamicClient *dynamicclient.DynamicClient
	cvpLister     v1alpha1.ClusterValidatePolicyLister
}

type ValidateResult struct {
	Reason string `json:"reason"`
	Valid  bool   `json:"valid"`
}

func NewValidateManager(dynamicClient *dynamicclient.DynamicClient, cvpLister v1alpha1.ClusterValidatePolicyLister) ValidateManager {
	return &validateManagerImpl{
		dynamicClient: dynamicClient,
		cvpLister:     cvpLister,
	}
}

func (m *validateManagerImpl) ApplyValidatePolicies(rawObj *unstructured.Unstructured, oldObj *unstructured.Unstructured, operation admissionv1.Operation) (*ValidateResult, error) {
	cvps, err := m.cvpLister.List(labels.Everything())
	if err != nil {
		klog.ErrorS(err, "Failed to list validate policies.", "resource", klog.KObj(rawObj), "operation", operation)
		return nil, err
	}

	if len(cvps) == 0 {
		klog.V(2).InfoS("No validate policy.", "resource", klog.KObj(rawObj), "operation", operation)
		return &ValidateResult{
			Valid: true,
		}, nil
	}

	for _, cvp := range cvps {
		result, err := m.applyValidatePolicy(cvp, rawObj, oldObj, operation)
		if err != nil {
			klog.ErrorS(err, "Failed to applyValidatePolicy.",
				"validatepolicy", cvp.Name, "resource", klog.KObj(rawObj), "operation", operation)
			return nil, err
		}

		if !result.Valid {
			return result, nil
		}
	}

	return &ValidateResult{
		Valid: true,
	}, nil
}

func (m *validateManagerImpl) applyValidatePolicy(cvp *policyv1alpha1.ClusterValidatePolicy, rawObj, oldObj *unstructured.Unstructured,
	operation admissionv1.Operation) (*ValidateResult, error) {
	if len(cvp.Spec.ResourceSelectors) > 0 && !utils.ResourceMatchSelectors(rawObj, cvp.Spec.ResourceSelectors...) {
		//no matched
		return &ValidateResult{Valid: true}, nil
	}

	for _, rule := range cvp.Spec.ValidateRules {
		if len(rule.TargetOperations) > 0 && !util.Exists(rule.TargetOperations, operation) {
			// no matched
			continue
		}
		if operation != admissionv1.Update {
			oldObj = nil
		}

		if rule.RenderedCue != "" {
			params := &cue.CueParams{
				Object:    rawObj,
				OldObject: oldObj,
			}

			result, err := m.executeRenderedCue(params, &rule, cvp.Name)
			if err != nil {
				klog.ErrorS(err, "Failed to execute rendered cue.",
					"validatepolicy", cvp.Name, "resource", klog.KObj(rawObj), "operation", operation)
				return nil, err
			}

			klog.V(2).InfoS("Applied validate policy.",
				"validatepolicy", cvp.Name, "resource", klog.KObj(rawObj), "operation", operation)
			if !result.Valid {
				return result, nil
			}
		}

		if rule.Cue != "" {
			result, err := executeCue(rawObj, oldObj, rule.Cue)
			if err != nil {
				klog.ErrorS(err, "Failed to apply validate policy.",
					"validatepolicy", cvp.Name, "resource", klog.KObj(rawObj), "operation", operation)
				return nil, err
			}
			klog.V(2).InfoS("Applied validate policy.",
				"validatepolicy", cvp.Name, "resource", klog.KObj(rawObj), "operation", operation)
			if !result.Valid {
				return result, nil
			}
		}
	}

	return &ValidateResult{
		Valid: true,
	}, nil
}

func (m *validateManagerImpl) executeRenderedCue(params *cue.CueParams, rule *policyv1alpha1.ValidateRuleWithOperation, cvpName string) (*ValidateResult, error) {
	var (
		extraParams *cue.CueParams
		err         error
	)

	if rule.Template != nil {
		extraParams, err = cue.BuildCueParamsViaValidatePolicy(m.dynamicClient, params.Object, rule.Template)
	} else if rule.PodAvailableBadge != nil {
		// support pab
		extraParams, err = cue.BuildCueParamsViaValidatePAB(m.dynamicClient, params.Object, rule.PodAvailableBadge)
	} else {
		return nil, fmt.Errorf("invalid validate policy")
	}
	if err != nil {
		klog.ErrorS(err, "Failed to build validate policy params.",
			"validatepolicy", cvpName, "resource", klog.KObj(params.Object))
		return nil, err
	}

	params.ExtraParams = extraParams.ExtraParams
	result, err := executeCueV2(rule.RenderedCue, []cue.Parameter{
		{
			Name:   utils.DataParameterName,
			Object: params,
		},
	})
	if err != nil {
		return nil, err
	}

	if rule.Template != nil && rule.Template.AffectMode == policyv1alpha1.AffectModeAllow {
		// if valid is true -> not match current policy -> reject operation
		// if valid is false -> matches policy -> allow operation
		result.Valid = !result.Valid
	}

	return result, nil
}

func executeCueV2(cueStr string, parameters []cue.Parameter) (*ValidateResult, error) {
	result := ValidateResult{
		Valid: true,
	}
	if err := cue.CueDoAndReturn(cueStr, parameters, utils.ValidateOutputName, &result); err != nil {
		return nil, err
	}

	klog.InfoS("v2", "result", result)
	return &result, nil
}

func executeCue(rawObj *unstructured.Unstructured, oldObj *unstructured.Unstructured, template string) (*ValidateResult, error) {
	result := ValidateResult{
		Valid: true,
	}
	parameters := []cue.Parameter{
		{
			Name:   utils.ObjectParameterName,
			Object: rawObj,
		},
	}

	if oldObj != nil {
		parameters = append(parameters, cue.Parameter{
			Name:   utils.OldObjectParameterName,
			Object: oldObj,
		})
	}
	if err := cue.CueDoAndReturn(template, parameters, utils.ValidateOutputName, &result); err != nil {
		return nil, err
	}

	klog.InfoS("v1", "result", result)
	return &result, nil
}
