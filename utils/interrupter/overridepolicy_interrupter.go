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
	"k8s.io/apimachinery/pkg/labels"
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

	o.handleValueRef(op, old, operation)
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

func (o *overridePolicyInterrupter) OnStartUp() error {
	list, err := o.lister.List(labels.Everything())
	if err != nil {
		return err
	}

	for _, policy := range list {
		o.handleValueRef(policy, nil, admissionv1.Create)
	}

	return nil
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

func (o *overridePolicyInterrupter) handleValueRef(policy, oldPolicy *policyv1alpha1.OverridePolicy, operation admissionv1.Operation) {
	newCallbackMap := o.getTokenCallbackMap(policy)

	var oldCallbackMap map[string]*tokenCallbackImpl
	if operation == admissionv1.Update && oldPolicy != nil {
		oldCallbackMap = o.getTokenCallbackMap(oldPolicy)
	}

	if operation == admissionv1.Create {
		for _, impl := range newCallbackMap {
			o.tokenManager.AddToken(impl.generator, impl)
		}
		return
	}

	if operation == admissionv1.Update {
		needUpdate, needRemove := compareCallbackMap(newCallbackMap, oldCallbackMap)
		for _, impl := range needRemove {
			o.tokenManager.RemoveToken(impl.generator, impl)
		}

		for _, impl := range needUpdate {
			o.tokenManager.AddToken(impl.generator, impl)
		}

		return
	}

	if operation == admissionv1.Delete {
		for _, impl := range newCallbackMap {
			o.tokenManager.RemoveToken(impl.generator, impl)
		}
	}
}

const (
	opTemplatePath = "/spec/overrideRules/%d/overriders/template"
	opAuthPath     = opTemplatePath + "/valueRef/http/auth/%s"
)

func (o *overridePolicyInterrupter) getTokenCallbackMap(policy *policyv1alpha1.OverridePolicy) map[string]*tokenCallbackImpl {
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
				getPolicy: o.getPolicy,
			}
		}

		cb.tokenPath = append(cb.tokenPath, fmt.Sprintf(opAuthPath, i, tokenKey))
		cb.expirePath = append(cb.expirePath, fmt.Sprintf(opAuthPath, i, expireAtKey))
		callbackMap[tg.ID()] = cb
	}

	for _, impl := range callbackMap {
		impl.id = fmt.Sprintf("%s/%s/%s", policy.GroupVersionKind(), policy.Namespace, policy.Name)
		impl.callback = o.genCallback(impl, policy.Namespace, policy.Name)
	}

	return callbackMap
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

func getTokenGeneratorFromRef(ref *policyv1alpha1.HttpDataRef) tokenmanager.TokenGenerator {
	if ref == nil || ref.Auth == nil || ref.Auth.StaticToken != "" || ref.Auth.AuthURL == "" {
		return nil
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

func copyCallbackMap(m map[string]*tokenCallbackImpl) map[string]*tokenCallbackImpl {
	nm := make(map[string]*tokenCallbackImpl)
	for s, impl := range m {
		nm[s] = impl
	}

	return nm
}

func compareCallbackMap(cur, old map[string]*tokenCallbackImpl) (update, remove map[string]*tokenCallbackImpl) {
	update = make(map[string]*tokenCallbackImpl)
	remove = make(map[string]*tokenCallbackImpl)

	for s, impl := range old {
		if _, ok := cur[s]; !ok {
			remove[s] = impl
		}
	}

	for s, impl := range cur {
		if _, ok := old[s]; !ok {
			update[s] = impl
			continue
		}

		// exist
		oldImpl := old[s]
		if !impl.generator.Equal(oldImpl.generator) {
			remove[s] = oldImpl
			update[s] = impl
		}
	}

	return
}
