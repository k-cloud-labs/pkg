package interrupter

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	jsonpatchv2 "gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/client/listers/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/utils/tokenmanager"
)

type overridePolicyInterrupter struct {
	*baseInterrupter
	tokenManager tokenmanager.TokenManager
	client       client.Client
	lister       v1alpha1.OverridePolicyLister
}

func (o *overridePolicyInterrupter) OnMutating(obj, oldObj *unstructured.Unstructured, operation admissionv1.Operation) ([]jsonpatchv2.JsonPatchOperation, error) {
	op := new(policyv1alpha1.OverridePolicy)
	if err := convertToPolicy(obj, op); err != nil {
		return nil, err
	}

	var old *policyv1alpha1.OverridePolicy
	if oldObj != nil {
		old = new(policyv1alpha1.OverridePolicy)
		if err := convertToPolicy(oldObj, old); err != nil {
			return nil, err
		}
	}

	// UPDATE op
	if old != nil {
		// no change
		if reflect.DeepEqual(op.Spec, old.Spec) {
			return nil, nil
		}
	}

	patches, err := o.patchOverridePolicy(op, operation)
	if err != nil {
		return nil, err
	}

	if err = o.handleValueRef(op, old, operation); err != nil {
		return nil, err
	}

	return patches, nil
}

func (o *overridePolicyInterrupter) OnValidating(obj, oldObj *unstructured.Unstructured, operation admissionv1.Operation) error {
	if operation == admissionv1.Delete {
		return nil
	}

	op := new(policyv1alpha1.OverridePolicy)
	if err := convertToPolicy(obj, op); err != nil {
		return err
	}

	var old *policyv1alpha1.OverridePolicy
	if oldObj != nil {
		old = new(policyv1alpha1.OverridePolicy)
		if err := convertToPolicy(oldObj, old); err != nil {
			return err
		}
	}

	// UPDATE op
	if old != nil {
		// no change
		if reflect.DeepEqual(op.Spec, old.Spec) {
			return nil
		}
	}

	return o.validateOverridePolicy(&op.Spec)
}

func NewOverridePolicyInterrupter(interrupter PolicyInterrupter, tm tokenmanager.TokenManager, client client.Client, lister v1alpha1.OverridePolicyLister) PolicyInterrupter {
	return &overridePolicyInterrupter{
		baseInterrupter: interrupter.(*baseInterrupter),
		tokenManager:    tm,
		client:          client,
		lister:          lister,
	}
}

func (o *overridePolicyInterrupter) validateOverridePolicy(objSpec *policyv1alpha1.OverridePolicySpec) error {
	for _, overrideRule := range objSpec.OverrideRules {
		if err := o.cueManager.Validate([]byte(overrideRule.Overriders.RenderedCue)); err != nil {
			return err
		}
	}

	return nil
}

func (o *overridePolicyInterrupter) patchOverridePolicy(policy *policyv1alpha1.OverridePolicy, operation admissionv1.Operation) ([]jsonpatchv2.JsonPatchOperation, error) {
	if operation == admissionv1.Delete {
		return nil, nil
	}

	patches := make([]jsonpatchv2.JsonPatchOperation, 0)
	for i, overrideRule := range policy.Spec.OverrideRules {
		if overrideRule.Overriders.Template == nil {
			continue
		}

		tmpl := overrideRule.Overriders.Template
		b, err := o.renderAndFormat(tmpl)
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

func (o *overridePolicyInterrupter) handleValueRef(policy, oldPolicy *policyv1alpha1.OverridePolicy, operation admissionv1.Operation) error {
	newCallbackMap, err := o.getTokenCallbackMap(policy)
	if err != nil {
		return err
	}

	if operation == admissionv1.Update && oldPolicy != nil {
		oldCallbackMap, err := o.getTokenCallbackMap(oldPolicy)
		if err != nil {
			return err
		}

		// remove old and add new
		for _, impl := range oldCallbackMap {
			o.tokenManager.RemoveToken(impl.generator, impl)
		}
	}

	if operation == admissionv1.Create || operation == admissionv1.Update {
		for _, impl := range newCallbackMap {
			o.tokenManager.AddToken(impl.generator, impl)
		}
		return nil
	}

	if operation == admissionv1.Delete {
		for _, impl := range newCallbackMap {
			o.tokenManager.RemoveToken(impl.generator, impl)
		}
		return nil
	}

	return nil
}

func (o *overridePolicyInterrupter) getTokenCallbackMap(policy *policyv1alpha1.OverridePolicy) (map[string]*tokenCallbackImpl, error) {
	callbackMap := make(map[string]*tokenCallbackImpl)
	for i, overrideRule := range policy.Spec.OverrideRules {
		if overrideRule.Overriders.Template == nil {
			continue
		}

		tmpl := overrideRule.Overriders.Template
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
				getPolicy: o.getPolicy,
			}
		}

		cb.tokenPath = append(cb.tokenPath, fmt.Sprintf("/spec/overrideRules/%d/overriders/template/valueRef/http/auth/token", i))
		cb.expirePath = append(cb.expirePath, fmt.Sprintf("/spec/overrideRules/%d/overriders/template/valueRef/http/auth/expireAt", i))
		callbackMap[tg.ID()] = cb
	}

	for _, impl := range callbackMap {
		impl.id = fmt.Sprintf("%s/%s/%s", policy.GroupVersionKind(), policy.Namespace, policy.Name)
		impl.callback = o.genCallback(impl, policy.Namespace, policy.Name)
	}

	return callbackMap, nil
}

func (o *overridePolicyInterrupter) getPolicy(namespace, name string) (client.Object, error) {
	return o.lister.OverridePolicies(namespace).Get(name)
}

func (o *overridePolicyInterrupter) genCallback(impl *tokenCallbackImpl, namespace, name string) func(token string, expireAt time.Time) error {
	return func(token string, expireAt time.Time) error {
		var patches = make([]jsonpatchv2.JsonPatchOperation, 0)
		for _, p := range impl.tokenPath {
			patches = append(patches,
				jsonpatchv2.JsonPatchOperation{
					Operation: "replace",
					Path:      p,
					Value:     token,
				},
			)
		}

		for _, p := range impl.expirePath {
			patches = append(patches,
				jsonpatchv2.JsonPatchOperation{
					Operation: "replace",
					Path:      p,
					Value:     metav1.NewTime(expireAt),
				},
			)
		}

		patchBytes, err := json.Marshal(patches)
		if err != nil {
			return nil
		}

		obj, err := impl.getPolicy(namespace, name)
		if err != nil {
			klog.ErrorS(err, "load override policy error", "namespace", namespace, "name", name)
			return err
		}

		return o.client.Patch(context.Background(), obj, client.RawPatch(types.JSONPatchType, patchBytes))
	}
}

func getTokenGeneratorFromRef(ref *policyv1alpha1.HttpDataRef) (tokenmanager.TokenGenerator, error) {
	if ref == nil {
		return nil, nil
	}

	if ref.Auth == nil || ref.Auth.StaticToken != "" {
		return nil, nil
	}

	if ref.Auth.AuthURL == "" {
		return nil, nil
	}

	return tokenmanager.NewTokenGenerator(ref.Auth.AuthURL, ref.Auth.Username, ref.Auth.Password, ref.Auth.ExpireDuration.Duration)
}

type tokenCallbackImpl struct {
	id         string
	callback   func(token string, expireAt time.Time) error
	generator  tokenmanager.TokenGenerator
	getPolicy  func(namespace, name string) (client.Object, error)
	tokenPath  []string
	expirePath []string
}

func (t *tokenCallbackImpl) ID() string {
	return t.id
}

func (t *tokenCallbackImpl) Callback(token string, expireAt time.Time) error {
	return t.callback(token, expireAt)
}
