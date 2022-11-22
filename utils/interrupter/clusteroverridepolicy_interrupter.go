package interrupter

import (
	"fmt"
	"reflect"

	jsonpatchv2 "gomodules.xyz/jsonpatch/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/client/listers/policy/v1alpha1"
)

type clusterOverridePolicyInterrupter struct {
	*overridePolicyInterrupter
	lister v1alpha1.ClusterOverridePolicyLister
}

func (c *clusterOverridePolicyInterrupter) OnMutating(obj, oldObj *unstructured.Unstructured) ([]jsonpatchv2.JsonPatchOperation, error) {
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

	return c.patchOverridePolicy(cop)
}

func (c *clusterOverridePolicyInterrupter) OnValidating(obj, oldObj *unstructured.Unstructured) error {
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

func NewClusterOverridePolicyInterrupter(opInterrupter PolicyInterrupter, lister v1alpha1.ClusterOverridePolicyLister) PolicyInterrupter {
	return &clusterOverridePolicyInterrupter{
		overridePolicyInterrupter: opInterrupter.(*overridePolicyInterrupter),
		lister:                    lister,
	}
}

func (c *clusterOverridePolicyInterrupter) patchOverridePolicy(policy *policyv1alpha1.ClusterOverridePolicy) ([]jsonpatchv2.JsonPatchOperation, error) {
	patches := make([]jsonpatchv2.JsonPatchOperation, 0)
	callbackMap := make(map[string]*tokenCallbackImpl)
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

		if tmpl.ValueRef == nil || tmpl.ValueRef.Http == nil {
			continue
		}

		tg, err := getTokenGeneratorFromRef(tmpl.ValueRef.Http)
		if err != nil {
			return nil, err
		}
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

		cb.tokenPath = append(cb.tokenPath, fmt.Sprintf("/sepc/overrideRules/%d/overriders/template/valueRef/http/auth/token", i))
		cb.expirePath = append(cb.tokenPath, fmt.Sprintf("/sepc/overrideRules/%d/overriders/template/valueRef/http/auth/expireAt", i))
		callbackMap[tg.ID()] = cb
	}

	for _, impl := range callbackMap {
		impl.id = fmt.Sprintf("%s/%s", policy.GroupVersionKind(), policy.Name)
		impl.callback = c.genCallback(impl, policy.Namespace, policy.Name)
		c.tokenManager.AddToken(impl.generator, impl)
	}

	return patches, nil
}

func (c *clusterOverridePolicyInterrupter) getPolicy(_, name string) (client.Object, error) {
	return c.lister.Get(name)
}
