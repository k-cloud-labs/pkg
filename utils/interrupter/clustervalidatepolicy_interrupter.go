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

type clusterValidatePolicyInterrupter struct {
	*baseInterrupter
	tokenManager tokenmanager.TokenManager
	client       client.Client
	lister       v1alpha1.ClusterValidatePolicyLister
}

func (v *clusterValidatePolicyInterrupter) OnMutating(obj, oldObj *unstructured.Unstructured, operation admissionv1.Operation) ([]jsonpatchv2.JsonPatchOperation, error) {
	cvp := new(policyv1alpha1.ClusterValidatePolicy)
	if err := convertToPolicy(obj, cvp); err != nil {
		return nil, err
	}

	var old *policyv1alpha1.ClusterValidatePolicy
	if oldObj != nil {
		old = new(policyv1alpha1.ClusterValidatePolicy)
		if err := convertToPolicy(oldObj, old); err != nil {
			return nil, err
		}
	}

	// UPDATE cvp
	if old != nil {
		// no change
		if reflect.DeepEqual(cvp.Spec, old.Spec) {
			return nil, nil
		}
	}

	patches, err := v.patchClusterValidatePolicy(cvp, operation)
	if err != nil {
		return nil, err
	}

	v.handleValueRef(cvp, old, operation)
	return patches, nil
}

func (v *clusterValidatePolicyInterrupter) OnValidating(obj, oldObj *unstructured.Unstructured, operation admissionv1.Operation) error {
	if operation == admissionv1.Delete {
		return nil
	}

	cvp := new(policyv1alpha1.ClusterValidatePolicy)
	if err := convertToPolicy(obj, cvp); err != nil {
		return err
	}

	var old *policyv1alpha1.ClusterValidatePolicy
	if oldObj != nil {
		old = new(policyv1alpha1.ClusterValidatePolicy)
		if err := convertToPolicy(oldObj, old); err != nil {
			return err
		}
	}

	// UPDATE cvp
	if old != nil {
		// no change
		if reflect.DeepEqual(cvp.Spec, old.Spec) {
			return nil
		}
	}

	return v.validateClusterValidatePolicy(cvp)
}

func (v *clusterValidatePolicyInterrupter) OnStartUp() error {
	list, err := v.lister.List(labels.Everything())
	if err != nil {
		return err
	}

	for _, policy := range list {
		v.handleValueRef(policy, nil, admissionv1.Create)
	}

	return nil
}

func NewClusterValidatePolicyInterrupter(interrupter PolicyInterrupter, tm tokenmanager.TokenManager,
	client client.Client, lister v1alpha1.ClusterValidatePolicyLister) PolicyInterrupter {
	return &clusterValidatePolicyInterrupter{
		baseInterrupter: interrupter.(*baseInterrupter),
		tokenManager:    tm,
		client:          client,
		lister:          lister,
	}
}

func (v *clusterValidatePolicyInterrupter) validateClusterValidatePolicy(obj *policyv1alpha1.ClusterValidatePolicy) error {
	for _, validateRule := range obj.Spec.ValidateRules {
		if validateRule.Template == nil && len(validateRule.RenderedCue) == 0 {
			continue
		}
		if err := v.cueManager.Validate([]byte(validateRule.RenderedCue)); err != nil {
			return err
		}
	}

	return nil
}

func (v *clusterValidatePolicyInterrupter) patchClusterValidatePolicy(policy *policyv1alpha1.ClusterValidatePolicy, operation admissionv1.Operation) ([]jsonpatchv2.JsonPatchOperation, error) {
	if operation == admissionv1.Delete {
		return nil, nil
	}

	patches := make([]jsonpatchv2.JsonPatchOperation, 0)
	for i, validateRule := range policy.Spec.ValidateRules {
		if validateRule.Template == nil {
			continue
		}

		if validateRule.Template.Condition != nil && validateRule.Template.Condition.AffectMode == "" {
			validateRule.Template.Condition.AffectMode = policyv1alpha1.AffectModeReject
			patches = append(patches, jsonpatchv2.JsonPatchOperation{
				Operation: "replace",
				Path:      fmt.Sprintf("/spec/validateRules/%d/template/condition/affectMode", i),
				Value:     policyv1alpha1.AffectModeReject,
			})
		}

		b, err := v.renderAndFormat(validateRule.Template)
		if err != nil {
			return nil, err
		}

		patches = append(patches, jsonpatchv2.JsonPatchOperation{
			Operation: "replace",
			Path:      fmt.Sprintf("/spec/validateRules/%d/renderedCue", i),
			Value:     string(b),
		})

	}

	return patches, nil
}

func (v *clusterValidatePolicyInterrupter) handleValueRef(policy, oldPolicy *policyv1alpha1.ClusterValidatePolicy, operation admissionv1.Operation) {
	newCallbackMap := v.getTokenCallbackMap(policy)

	var oldCallbackMap map[string]*tokenCallbackImpl
	if operation == admissionv1.Update && oldPolicy != nil {
		oldCallbackMap = v.getTokenCallbackMap(oldPolicy)
	}

	if operation == admissionv1.Create {
		for _, impl := range newCallbackMap {
			v.tokenManager.AddToken(impl.generator, impl)
		}
		return
	}

	if operation == admissionv1.Update {
		needUpdate, needRemove := compareCallbackMap(newCallbackMap, oldCallbackMap)
		for _, impl := range needRemove {
			v.tokenManager.RemoveToken(impl.generator, impl)
		}

		for _, impl := range needUpdate {
			v.tokenManager.AddToken(impl.generator, impl)
		}

		return
	}

	if operation == admissionv1.Delete {
		for _, impl := range newCallbackMap {
			v.tokenManager.RemoveToken(impl.generator, impl)
		}
	}
}

const (
	cvpTemplatePath   = "/spec/validateRules/%d/template"
	dataRefKey        = "dataRef"
	valueRefKey       = "valueRef"
	tokenKey          = "token"
	expireAtKey       = "expireAt"
	conditionAuthPath = cvpTemplatePath + "/condition/%s/http/auth/%s"
)

func (v *clusterValidatePolicyInterrupter) getTokenCallbackMap(policy *policyv1alpha1.ClusterValidatePolicy) map[string]*tokenCallbackImpl {
	callbackMap := make(map[string]*tokenCallbackImpl)
	checkAndAppend := func(ref *policyv1alpha1.HttpDataRef, tokenPath, expirePath string) {
		tg := getTokenGeneratorFromRef(ref)
		if tg == nil {
			return
		}

		cb, ok := callbackMap[tg.ID()]
		if !ok {
			cb = &tokenCallbackImpl{
				generator: tg,
				getPolicy: v.getPolicy,
			}
		}

		cb.tokenPath = append(cb.tokenPath, tokenPath)
		cb.expirePath = append(cb.expirePath, expirePath)
		callbackMap[tg.ID()] = cb
	}

	for i, rule := range policy.Spec.ValidateRules {
		if rule.Template == nil {
			continue
		}

		tmpl := rule.Template
		// condition
		if tmpl.Condition != nil {
			if tmpl.Condition.DataRef != nil && tmpl.Condition.DataRef.Http != nil {
				checkAndAppend(tmpl.Condition.DataRef.Http,
					fmt.Sprintf(conditionAuthPath, i, dataRefKey, tokenKey),
					fmt.Sprintf(conditionAuthPath, i, dataRefKey, expireAtKey),
				)
			}

			if tmpl.Condition.ValueRef != nil && tmpl.Condition.ValueRef.Http != nil {
				checkAndAppend(tmpl.Condition.DataRef.Http,
					fmt.Sprintf(conditionAuthPath, i, valueRefKey, tokenKey),
					fmt.Sprintf(conditionAuthPath, i, valueRefKey, expireAtKey),
				)
			}
		}
	}

	for _, impl := range callbackMap {
		impl.id = fmt.Sprintf("%s/%s", policy.GroupVersionKind(), policy.Name)
		impl.callback = v.genCallback(impl, policy.Namespace, policy.Name)
	}

	return callbackMap
}

func (v *clusterValidatePolicyInterrupter) getPolicy(_, name string) (client.Object, error) {
	return v.lister.Get(name)
}

func (v *clusterValidatePolicyInterrupter) genCallback(impl *tokenCallbackImpl, namespace, name string) func(token string, expireAt time.Time) error {
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
			klog.ErrorS(err, "load cluster validate policy error", "namespace", namespace, "name", name)
			return err
		}

		klog.V(4).InfoS("before patch cvp", "cvp", obj.GetName(), "patchBytes", string(patchBytes))
		return v.client.Patch(context.Background(), obj, client.RawPatch(types.JSONPatchType, patchBytes))
	}
}
