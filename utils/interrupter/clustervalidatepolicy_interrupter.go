package interrupter

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	jsonpatchv2 "gomodules.xyz/jsonpatch/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

func (v *clusterValidatePolicyInterrupter) OnMutating(obj, oldObj *unstructured.Unstructured) ([]jsonpatchv2.JsonPatchOperation, error) {
	cvp := new(policyv1alpha1.ClusterValidatePolicy)
	if err := convertToPolicy(obj, cvp); err != nil {
		return nil, err
	}

	var old *policyv1alpha1.ClusterOverridePolicy
	if oldObj != nil {
		old = new(policyv1alpha1.ClusterOverridePolicy)
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

	return v.patchClusterValidatePolicy(cvp)
}

func (v *clusterValidatePolicyInterrupter) OnValidating(obj, oldObj *unstructured.Unstructured) error {
	cvp := new(policyv1alpha1.ClusterValidatePolicy)
	if err := convertToPolicy(obj, cvp); err != nil {
		return err
	}

	var old *policyv1alpha1.ClusterOverridePolicy
	if oldObj != nil {
		old = new(policyv1alpha1.ClusterOverridePolicy)
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
		if validateRule.Template == nil || len(validateRule.RenderedCue) == 0 {
			continue
		}
		if err := v.cueManager.Validate([]byte(validateRule.RenderedCue)); err != nil {
			return err
		}
	}

	return nil
}

func (v *clusterValidatePolicyInterrupter) patchClusterValidatePolicy(policy *policyv1alpha1.ClusterValidatePolicy) ([]jsonpatchv2.JsonPatchOperation, error) {
	patches := make([]jsonpatchv2.JsonPatchOperation, 0)
	var (
		callbackMap = make(map[string]*tokenCallbackImpl)
	)
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
		if validateRule.Template.PodAvailableBadge != nil &&
			validateRule.Template.PodAvailableBadge.ReplicaReference == nil {
			validateRule.Template.PodAvailableBadge.ReplicaReference = &policyv1alpha1.ReplicaResourceRefer{
				From:               policyv1alpha1.FromOwnerReference,
				TargetReplicaPath:  "/spec/replicas",
				CurrentReplicaPath: "/status/replicas",
			}

			patches = append(patches, jsonpatchv2.JsonPatchOperation{
				Operation: "replace",
				Path:      fmt.Sprintf("/spec/validateRules/%d/template/podAvailableBadge/replicaReference", i),
				Value:     validateRule.Template.PodAvailableBadge.ReplicaReference,
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

		if err = v.pickTokenRefFromValidateTemplate(policy, callbackMap, i); err != nil {
			return nil, err
		}
	}
	for _, impl := range callbackMap {
		impl.id = fmt.Sprintf("%s/%s", policy.GroupVersionKind(), policy.Name)
		impl.callback = v.genCallback(impl, policy.Namespace, policy.Name)
		v.tokenManager.AddToken(impl.generator, impl)
	}

	return patches, nil
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
			klog.ErrorS(err, "load override policy error", "namespace", namespace, "name", name)
			return err
		}

		return v.client.Status().Patch(context.Background(), obj, client.RawPatch(types.JSONPatchType, patchBytes))
	}
}

func (v *clusterValidatePolicyInterrupter) pickTokenRefFromValidateTemplate(policy *policyv1alpha1.ClusterValidatePolicy,
	m map[string]*tokenCallbackImpl, ruleIdx int) error {
	tmpl := policy.Spec.ValidateRules[ruleIdx].Template
	if tmpl == nil {
		return nil
	}

	checkAndAppend := func(ref *policyv1alpha1.HttpDataRef, tokenPath, expirePath string) error {
		tg, err := getTokenGeneratorFromRef(tmpl.Condition.DataRef.Http)
		if err != nil {
			return err
		}
		if tg == nil {
			return nil
		}

		cb, ok := m[tg.ID()]
		if !ok {
			cb = &tokenCallbackImpl{
				generator: tg,
				getPolicy: v.getPolicy,
			}
		}

		cb.tokenPath = append(cb.tokenPath, tokenPath)
		cb.expirePath = append(cb.tokenPath, expirePath)
		m[tg.ID()] = cb
		return nil
	}

	// condition
	if tmpl.Condition != nil {
		if tmpl.Condition.DataRef != nil && tmpl.Condition.DataRef.Http != nil {
			err := checkAndAppend(tmpl.Condition.DataRef.Http,
				fmt.Sprintf("/sepc/validateRules/%d/template/condition/dataRef/http/auth/token", ruleIdx),
				fmt.Sprintf("/sepc/validateRules/%d/template/condition/dataRef/http/auth/expireAt", ruleIdx),
			)
			if err != nil {
				return err
			}
		}

		if tmpl.Condition.ValueRef != nil && tmpl.Condition.ValueRef.Http != nil {
			err := checkAndAppend(tmpl.Condition.DataRef.Http,
				fmt.Sprintf("/sepc/validateRules/%d/template/condition/valueRef/http/auth/token", ruleIdx),
				fmt.Sprintf("/sepc/validateRules/%d/template/condition/valueRef/http/auth/expireAt", ruleIdx),
			)
			if err != nil {
				return err
			}
		}
	}

	// pab
	if tmpl.PodAvailableBadge != nil && tmpl.PodAvailableBadge.ReplicaReference != nil &&
		tmpl.PodAvailableBadge.ReplicaReference.Http != nil {
		err := checkAndAppend(tmpl.PodAvailableBadge.ReplicaReference.Http,
			fmt.Sprintf("/sepc/validateRules/%d/template/AvailableBadge/replicaReference/http/auth/token", ruleIdx),
			fmt.Sprintf("/sepc/validateRules/%d/template/AvailableBadge/replicaReference/http/auth/expireAt", ruleIdx),
		)
		if err != nil {
			return err
		}
	}

	return nil
}
