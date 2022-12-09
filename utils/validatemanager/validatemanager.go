package validatemanager

import (
	"bytes"
	"encoding/json"
	"fmt"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/client/listers/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/utils"
	"github.com/k-cloud-labs/pkg/utils/cue"
	"github.com/k-cloud-labs/pkg/utils/dynamiclister"
	"github.com/k-cloud-labs/pkg/utils/metrics"
	"github.com/k-cloud-labs/pkg/utils/util"
)

// ValidateManager managers validate policies for operation
type ValidateManager interface {
	// ApplyValidatePolicies validate the object if one or more matched validate policy exist.
	ApplyValidatePolicies(obj *unstructured.Unstructured, oldObj *unstructured.Unstructured, operation admissionv1.Operation) (*ValidateResult, error)
}

type validateManagerImpl struct {
	dynamicClient dynamiclister.DynamicResourceLister
	cvpLister     v1alpha1.ClusterValidatePolicyLister
}

type ValidateResult struct {
	Reason string `json:"reason"`
	Valid  bool   `json:"valid"`
}

func NewValidateManager(dynamicClient dynamiclister.DynamicResourceLister, cvpLister v1alpha1.ClusterValidatePolicyLister) ValidateManager {
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

	metrics.ValidatePolicyMatched(cvp.Name, rawObj.GroupVersionKind())
	klog.V(4).InfoS("resource matched a validate policy", "operation", operation, "policy", cvp.GroupVersionKind(),
		"resource", fmt.Sprintf("%v/%v/%v", rawObj.GroupVersionKind(), rawObj.GetNamespace(), rawObj.GetName()))
	for _, rule := range cvp.Spec.ValidateRules {
		if len(rule.TargetOperations) > 0 && !util.Exists(rule.TargetOperations, operation) {
			// no matched
			continue
		}
		if operation != admissionv1.Update {
			oldObj = nil
		}

		if rule.Template != nil && rule.RenderedCue != "" {
			params := &cue.CueParams{
				Object:    rawObj,
				OldObject: oldObj,
			}

			result, err := m.executeTemplate(params, &rule, cvp.Name)
			if err != nil {
				klog.ErrorS(err, "Failed to execute rendered cue.",
					"validatepolicy", cvp.Name, "resource", klog.KObj(rawObj), "operation", operation)
				return nil, err
			}

			if result != nil {
				klog.V(2).InfoS("Applied validate policy.",
					"validatepolicy", cvp.Name, "resource", klog.KObj(rawObj), "operation", operation)
				if !result.Valid {
					metrics.ValidatePolicyReject(cvp.Name, rawObj.GroupVersionKind())
					return result, nil
				}
			}
		}

		if rule.Cue != "" {
			result, err := executeCue(rawObj, oldObj, rule.Cue)
			if err != nil {
				metrics.PolicyGotError(rawObj.GetName(), rawObj.GroupVersionKind(), metrics.ErrorTypeCueExecute)
				klog.ErrorS(err, "Failed to apply validate policy.",
					"validatepolicy", cvp.Name, "resource", klog.KObj(rawObj), "operation", operation)
				return nil, err
			}
			klog.V(2).InfoS("Applied validate policy.",
				"validatepolicy", cvp.Name, "resource", klog.KObj(rawObj), "operation", operation)
			if !result.Valid {
				metrics.ValidatePolicyReject(cvp.Name, rawObj.GroupVersionKind())
				return result, nil
			}
		}
	}

	return &ValidateResult{
		Valid: true,
	}, nil
}

func (m *validateManagerImpl) executeTemplate(params *cue.CueParams, rule *policyv1alpha1.ValidateRuleWithOperation, cvpName string) (*ValidateResult, error) {
	extraParams, err := cue.BuildCueParamsViaValidatePolicy(m.dynamicClient, params.Object, rule.Template)
	if err != nil {
		metrics.PolicyGotError(cvpName, params.Object.GroupVersionKind(), metrics.ErrTypePrepareCueParams)
		klog.ErrorS(err, "Failed to build validate policy params.",
			"validatepolicy", cvpName, "resource", klog.KObj(params.Object))
		return nil, err
	}

	if klog.V(4).Enabled() {
		buf := &bytes.Buffer{}
		enc := json.NewEncoder(buf)
		enc.SetIndent("", "\t")
		if err := enc.Encode(extraParams); err != nil {
			klog.ErrorS(err, "encode")
		}
		klog.V(4).InfoS("before execute", "params", buf.String(), "cue", rule.RenderedCue)

	}
	params.ExtraParams = extraParams.ExtraParams
	result, err := executeCueV2(rule.RenderedCue, []cue.Parameter{
		{
			Name:   utils.DataParameterName,
			Object: params,
		},
	})
	if err != nil {
		metrics.PolicyGotError(cvpName, params.Object.GroupVersionKind(), metrics.ErrorTypeCueExecute)
		return nil, err
	}

	if rule.Template != nil &&
		rule.Template.Condition != nil &&
		rule.Template.Condition.AffectMode == policyv1alpha1.AffectModeAllow {
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

func getPodPhase(obj *unstructured.Unstructured) corev1.PodPhase {
	if obj == nil {
		return ""
	}

	if obj.GetKind() != "Pod" {
		return ""
	}

	val, ok, err := unstructured.NestedString(obj.Object, "status", "phase")
	if err != nil {
		klog.ErrorS(err, "load pod phase failed", "name", obj.GetNamespace()+"/"+obj.GetName())
		return ""
	}

	if !ok {
		return ""
	}

	return corev1.PodPhase(val)
}
