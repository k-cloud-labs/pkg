package overridemanager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	jsonpatch "github.com/evanphx/json-patch"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/client/listers/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/utils"
	"github.com/k-cloud-labs/pkg/utils/cue"
	"github.com/k-cloud-labs/pkg/utils/dynamiclister"
	"github.com/k-cloud-labs/pkg/utils/util"
)

// OverrideManager managers override policies for operation
type OverrideManager interface {
	// ApplyOverridePolicies overrides the object if one or more matched override policies exist.
	// For cluster scoped resource:
	// - Apply ClusterOverridePolicy by policies name in ascending
	// For namespaced scoped resource, apply order is:
	// - First apply ClusterOverridePolicy;
	// - Then apply OverridePolicy;
	ApplyOverridePolicies(rawObj, oldObj *unstructured.Unstructured, operation admissionv1.Operation) (appliedCOPs *AppliedOverrides, appliedOPs *AppliedOverrides, err error)
}

// GeneralOverridePolicy is an abstract object of ClusterOverridePolicy and OverridePolicy
type GeneralOverridePolicy interface {
	// GetName returns the name of OverridePolicy
	GetName() string
	// GetNamespace returns the namespace of OverridePolicy
	GetNamespace() string
	// GetOverridePolicySpec returns the OverridePolicySpec of OverridePolicy
	GetOverridePolicySpec() policyv1alpha1.OverridePolicySpec
}

// overrideOption define the JSONPatch operator
type overrideOption struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

type policyOverriders struct {
	name       string
	namespace  string
	overriders policyv1alpha1.Overriders
}

type overrideManagerImpl struct {
	dynamicLister dynamiclister.DynamicResourceLister
	opLister      v1alpha1.OverridePolicyLister
	copLister     v1alpha1.ClusterOverridePolicyLister
}

func NewOverrideManager(dynamicClient dynamiclister.DynamicResourceLister, copLister v1alpha1.ClusterOverridePolicyLister, opLister v1alpha1.OverridePolicyLister) OverrideManager {
	return &overrideManagerImpl{
		dynamicLister: dynamicClient,
		opLister:      opLister,
		copLister:     copLister,
	}
}

func (o *overrideManagerImpl) ApplyOverridePolicies(rawObj, oldObj *unstructured.Unstructured, operation admissionv1.Operation) (*AppliedOverrides, *AppliedOverrides, error) {
	var (
		appliedCOPs *AppliedOverrides
		appliedOPs  *AppliedOverrides
		err         error
	)

	appliedCOPs, err = o.applyClusterOverridePolicies(rawObj, oldObj, operation)
	if err != nil {
		klog.ErrorS(err, "Failed to apply cluster override policies.")
		return nil, nil, err
	}

	if rawObj.GetNamespace() != "" {
		// Apply namespace scoped override policies
		appliedOPs, err = o.applyOverridePolicies(rawObj, oldObj, operation)
		if err != nil {
			klog.ErrorS(err, "Failed to apply override policies.")
			return nil, nil, err
		}
	}

	return appliedCOPs, appliedOPs, nil
}

func (o *overrideManagerImpl) applyClusterOverridePolicies(rawObj, oldObj *unstructured.Unstructured, operation admissionv1.Operation) (*AppliedOverrides, error) {
	cops, err := o.copLister.List(labels.Everything())
	if err != nil {
		klog.ErrorS(err, "Failed to list cluster override policies.", "resource", klog.KObj(rawObj), "operation", operation)
		return nil, err
	}

	if len(cops) == 0 {
		klog.V(2).InfoS("No cluster override policy in current cluster.")
		return nil, nil
	}

	items := make([]GeneralOverridePolicy, 0, len(cops))
	for i := range cops {
		items = append(items, cops[i])
	}

	matchingPolicyOverriders := o.getOverridersFromOverridePolicies(items, rawObj, operation)
	if len(matchingPolicyOverriders) == 0 {
		klog.V(2).InfoS("No cluster override policy.", "resource", klog.KObj(rawObj), "operation", operation)
		return nil, nil
	}

	appliedOverrides := &AppliedOverrides{}
	for _, p := range matchingPolicyOverriders {
		if err := o.applyPolicyOverriders(rawObj, oldObj, p.overriders); err != nil {
			klog.ErrorS(err, "Failed to apply cluster overriders.", "clusteroverridepolicy", p.name, "resource", klog.KObj(rawObj), "operation", operation)
			return nil, err
		}
		klog.V(2).InfoS("Applied cluster overriders.", "clusteroverridepolicy", p.name, "resource", klog.KObj(rawObj), "operation", operation)
		appliedOverrides.Add(p.name, p.overriders)
	}

	return appliedOverrides, nil
}

func (o *overrideManagerImpl) applyOverridePolicies(rawObj, oldObj *unstructured.Unstructured, operation admissionv1.Operation) (*AppliedOverrides, error) {
	ops, err := o.opLister.List(labels.Everything())
	if err != nil {
		klog.ErrorS(err, "Failed to list override policies.", "namespace", rawObj.GetNamespace(), "resource", klog.KObj(rawObj), "operation", operation)
		return nil, err
	}

	if len(ops) == 0 {
		return nil, nil
	}

	items := make([]GeneralOverridePolicy, 0, len(ops))
	for i := range ops {
		items = append(items, ops[i])
	}

	matchingPolicyOverriders := o.getOverridersFromOverridePolicies(items, rawObj, operation)
	if len(matchingPolicyOverriders) == 0 {
		klog.V(2).InfoS("No override policy.", "resource", klog.KObj(rawObj), "operation", operation)
		return nil, nil
	}
	klog.V(4).InfoS("matched override polices", "count", len(ops))

	appliedOverriders := &AppliedOverrides{}
	for _, p := range matchingPolicyOverriders {
		if err := o.applyPolicyOverriders(rawObj, oldObj, p.overriders); err != nil {
			klog.ErrorS(err, "Failed to apply overriders.",
				"overridepolicy", fmt.Sprintf("%s/%s", p.namespace, p.name), "resource", klog.KObj(rawObj), "operation", operation)
			return nil, fmt.Errorf("appling policy(%v/%v) err=%v", p.namespace, p.name, err)
		}
		klog.V(2).InfoS("Applied overriders", "overridepolicy", fmt.Sprintf("%s/%s", p.namespace, p.name), "resource", klog.KObj(rawObj), "operation", operation)
		appliedOverriders.Add(p.name, p.overriders)
	}

	return appliedOverriders, nil
}

func (o *overrideManagerImpl) getOverridersFromOverridePolicies(policies []GeneralOverridePolicy, resource *unstructured.Unstructured, operation admissionv1.Operation) []policyOverriders {
	resourceMatchingPolicies := make([]GeneralOverridePolicy, 0)

	for _, policy := range policies {
		if len(policy.GetOverridePolicySpec().ResourceSelectors) == 0 {
			resourceMatchingPolicies = append(resourceMatchingPolicies, policy)
			continue
		}

		if utils.ResourceMatchSelectors(resource, policy.GetOverridePolicySpec().ResourceSelectors...) {
			resourceMatchingPolicies = append(resourceMatchingPolicies, policy)
		}
	}

	matchingPolicyOverriders := make([]policyOverriders, 0)

	for _, policy := range resourceMatchingPolicies {
		for _, rule := range policy.GetOverridePolicySpec().OverrideRules {
			if len(rule.TargetOperations) == 0 || util.Exists(rule.TargetOperations, operation) {
				matchingPolicyOverriders = append(matchingPolicyOverriders, policyOverriders{
					name:       policy.GetName(),
					namespace:  policy.GetNamespace(),
					overriders: rule.Overriders,
				})
			}
		}
	}

	sort.Slice(matchingPolicyOverriders, func(i, j int) bool {
		return matchingPolicyOverriders[i].name < matchingPolicyOverriders[j].name
	})

	return matchingPolicyOverriders
}

// applyPolicyOverriders applies OverridePolicy/ClusterOverridePolicy overriders to target object
func (o *overrideManagerImpl) applyPolicyOverriders(rawObj, oldObj *unstructured.Unstructured, overriders policyv1alpha1.Overriders) error {
	if overriders.Template != nil && overriders.RenderedCue != "" {
		p, err := cue.BuildCueParamsViaOverridePolicy(o.dynamicLister, rawObj, overriders.Template)
		if err != nil {
			return fmt.Errorf("BuildCueParamsViaOverridePolicy error=%w", err)
		}
		p.Object = rawObj
		p.OldObject = oldObj
		if p.OldObject == nil {
			p.OldObject = &unstructured.Unstructured{Object: map[string]interface{}{}}
		}
		params := []cue.Parameter{
			{
				Object: p,
				Name:   utils.DataParameterName,
			},
		}

		patches, err := executeCueV2(overriders.RenderedCue, params)
		if err != nil {
			return err
		}

		if err := applyJSONPatch(rawObj, patches); err != nil {
			return err
		}
	}
	if overriders.Cue != "" {
		patches, err := executeCue(rawObj, overriders.Cue)
		if err != nil {
			return err
		}
		if err := applyJSONPatch(rawObj, *patches); err != nil {
			return err
		}
	}

	return applyJSONPatch(rawObj, parseJSONPatchesByPlaintext(overriders.Plaintext))
}

// applyJSONPatch applies the override on to the given unstructured object.
func applyJSONPatch(obj *unstructured.Unstructured, overrides []overrideOption) error {
	jsonPatchBytes, err := json.Marshal(overrides)
	if err != nil {
		return err
	}

	patch, err := jsonpatch.DecodePatch(jsonPatchBytes)
	if err != nil {
		return err
	}

	objectJSONBytes, err := obj.MarshalJSON()
	if err != nil {
		return err
	}

	patchedObjectJSONBytes, err := patch.Apply(objectJSONBytes)
	if err != nil {
		return err
	}

	err = obj.UnmarshalJSON(patchedObjectJSONBytes)
	return err
}

func parseJSONPatchesByPlaintext(overriders []policyv1alpha1.PlaintextOverrider) []overrideOption {
	patches := make([]overrideOption, 0, len(overriders))
	for i := range overriders {
		patches = append(patches, overrideOption{
			Op:    string(overriders[i].Operator),
			Path:  overriders[i].Path,
			Value: overriders[i].Value,
		})
	}
	return patches
}

func executeCueV2(cueStr string, parameters []cue.Parameter) ([]overrideOption, error) {
	result := make([]overrideOption, 0)
	if err := cue.CueDoAndReturn(cueStr, parameters, utils.OverrideOutputName, &result); err != nil {
		klog.ErrorS(err, "execute cue error", "cue", cueStr, "params", parameters)
		if klog.V(4).Enabled() {
			buf := &bytes.Buffer{}
			enc := json.NewEncoder(buf)
			enc.SetIndent("", "\t")

			if err := enc.Encode(parameters); err != nil {
				return nil, err
			}
			klog.V(4).InfoS("debug parameters", "params", buf.String(), "err", err.Error())
		}
		return nil, err
	}

	if klog.V(4).Enabled() {
		buf, err := marshalIndent(result)
		if err != nil {
			return nil, err
		}

		buf2, err := marshalIndent(parameters)
		if err != nil {
			return nil, err
		}

		klog.V(4).InfoS("cue execute result", "params", buf2.String(), "results", buf.String())
	}

	return result, nil
}

func marshalIndent(v any) (*bytes.Buffer, error) {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetIndent("", "\t")

	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf, nil
}

func executeCue(rawObj *unstructured.Unstructured, template string) (*[]overrideOption, error) {
	result := make([]overrideOption, 0)
	if err := cue.CueDoAndReturn(template, []cue.Parameter{{Name: utils.ObjectParameterName, Object: rawObj}}, utils.OverrideOutputName, &result); err != nil {
		return nil, err
	}

	return &result, nil
}
