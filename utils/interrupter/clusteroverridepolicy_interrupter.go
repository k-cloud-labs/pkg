package interrupter

import (
	"fmt"
	"reflect"

	jsonpatchv2 "gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/client/listers/policy/v1alpha1"
)

type clusterOverridePolicyInterrupter struct {
	*overridePolicyInterrupter
	lister v1alpha1.ClusterOverridePolicyLister
}

func (c *clusterOverridePolicyInterrupter) OnMutating(obj, oldObj *unstructured.Unstructured, operation admissionv1.Operation) ([]jsonpatchv2.JsonPatchOperation, error) {
	cop := new(policyv1alpha1.ClusterOverridePolicy)
	if err := convertToPolicy(obj, cop); err != nil {
		return nil, err
	}

	var old *policyv1alpha1.ClusterOverridePolicy
	if oldObj != nil {
		old = new(policyv1alpha1.ClusterOverridePolicy)
		if err := convertToPolicy(oldObj, old); err != nil {
			return nil, err
		}
	}

	// UPDATE op
	if old != nil {
		// no change
		if reflect.DeepEqual(cop.Spec, old.Spec) {
			return nil, nil
		}
	}

	patches, err := c.patchOverridePolicy(cop, operation)
	if err != nil {
		return nil, err
	}

	c.handleValueRef(cop, old, operation)
	return patches, nil
}

func (c *clusterOverridePolicyInterrupter) OnValidating(obj, oldObj *unstructured.Unstructured, operation admissionv1.Operation) error {
	if operation == admissionv1.Delete {
		return nil
	}
	cop := new(policyv1alpha1.ClusterOverridePolicy)
	if err := convertToPolicy(obj, cop); err != nil {
		return err
	}

	var old *policyv1alpha1.ClusterOverridePolicy
	if oldObj != nil {
		old = new(policyv1alpha1.ClusterOverridePolicy)
		if err := convertToPolicy(oldObj, old); err != nil {
			return err
		}
	}

	// UPDATE cop
	if old != nil {
		// no change
		if reflect.DeepEqual(cop.Spec, old.Spec) {
			return nil
		}
	}

	return c.validateOverridePolicy(&cop.Spec)
}

func (c *clusterOverridePolicyInterrupter) OnStartUp() error {
	list, err := c.lister.List(labels.Everything())
	if err != nil {
		return err
	}

	for _, policy := range list {
		c.handleValueRef(policy, nil, admissionv1.Create)
	}

	return nil
}

func NewClusterOverridePolicyInterrupter(opInterrupter PolicyInterrupter, lister v1alpha1.ClusterOverridePolicyLister) PolicyInterrupter {
	return &clusterOverridePolicyInterrupter{
		overridePolicyInterrupter: opInterrupter.(*overridePolicyInterrupter),
		lister:                    lister,
	}
}

func (c *clusterOverridePolicyInterrupter) patchOverridePolicy(policy *policyv1alpha1.ClusterOverridePolicy, operation admissionv1.Operation) ([]jsonpatchv2.JsonPatchOperation, error) {
	if operation == admissionv1.Delete {
		return nil, nil
	}

	patches := make([]jsonpatchv2.JsonPatchOperation, 0)
	for i, overrideRule := range policy.Spec.OverrideRules {
		if overrideRule.Overriders.Template == nil {
			continue
		}

		tmpl := overrideRule.Overriders.Template
		b, err := c.renderAndFormat(tmpl)
		if err != nil {
			return nil, err
		}

		patches = append(patches, jsonpatchv2.JsonPatchOperation{
			Operation: "replace",
			Path:      fmt.Sprintf("/spec/overrideRules/%d/overriders/renderedCue", i),
			Value:     string(b),
		})
	}

	return patches, nil
}

func (c *clusterOverridePolicyInterrupter) handleValueRef(policy, oldPolicy *policyv1alpha1.ClusterOverridePolicy, operation admissionv1.Operation) {
	newCallbackMap := c.getTokenCallbackMap(policy)

	var oldCallbackMap map[string]*tokenCallbackImpl
	if operation == admissionv1.Update && oldPolicy != nil {
		oldCallbackMap = c.getTokenCallbackMap(oldPolicy)
	}

	if operation == admissionv1.Create {
		for _, impl := range newCallbackMap {
			c.tokenManager.AddToken(impl.generator, impl)
		}
		return
	}

	if operation == admissionv1.Update {
		needUpdate, needRemove := compareCallbackMap(newCallbackMap, oldCallbackMap)
		for _, impl := range needRemove {
			c.tokenManager.RemoveToken(impl.generator, impl)
		}

		for _, impl := range needUpdate {
			c.tokenManager.AddToken(impl.generator, impl)
		}

		return
	}

	if operation == admissionv1.Delete {
		for _, impl := range newCallbackMap {
			c.tokenManager.RemoveToken(impl.generator, impl)
		}
	}
}

func (c *clusterOverridePolicyInterrupter) getTokenCallbackMap(policy *policyv1alpha1.ClusterOverridePolicy) map[string]*tokenCallbackImpl {
	callbackMap := make(map[string]*tokenCallbackImpl)
	for i, overrideRule := range policy.Spec.OverrideRules {
		if overrideRule.Overriders.Template == nil {
			continue
		}

		tmpl := overrideRule.Overriders.Template
		if tmpl.ValueRef == nil || tmpl.ValueRef.Http == nil {
			continue
		}

		tg := getTokenGeneratorFromRef(tmpl.ValueRef.Http)
		if tg == nil {
			continue
		}

		cb, ok := callbackMap[tg.ID()]
		if !ok {
			cb = &tokenCallbackImpl{
				generator: tg,
				getPolicy: c.getPolicy,
			}
		}

		cb.tokenPath = append(cb.tokenPath, fmt.Sprintf(opAuthPath, i, tokenKey))
		cb.expirePath = append(cb.tokenPath, fmt.Sprintf(opAuthPath, i, expireAtKey))
		callbackMap[tg.ID()] = cb
	}

	for _, impl := range callbackMap {
		impl.id = fmt.Sprintf("%s/%s/%s", policy.GroupVersionKind(), policy.Namespace, policy.Name)
		impl.callback = c.genCallback(impl, policy.Namespace, policy.Name)
	}

	return callbackMap
}

func (c *clusterOverridePolicyInterrupter) getPolicy(_, name string) (client.Object, error) {
	return c.lister.Get(name)
}
