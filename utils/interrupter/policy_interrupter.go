package interrupter

import (
	"strings"

	jsonpatchv2 "gomodules.xyz/jsonpatch/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
)

// PolicyInterrupter defines interrupt process for policy change
// It validate and mutate policy.
type PolicyInterrupter interface {
	// OnMutating called on "/mutating" api to complete policy
	// return nil means obj is not defined policy
	OnMutating(obj, oldObj *unstructured.Unstructured) ([]jsonpatchv2.JsonPatchOperation, error)
	// OnValidating called on "/validating" api to validate policy
	// return nil means obj is not defined policy or no invalid field
	OnValidating(obj, oldObj *unstructured.Unstructured) error
}

type policyInterrupterImpl struct {
	overridePolicyInterrupter        PolicyInterrupter
	clusterOverridePolicyInterrupter PolicyInterrupter
	clusterValidatePolicyInterrupter PolicyInterrupter
}

func (p *policyInterrupterImpl) OnMutating(obj, oldObj *unstructured.Unstructured) ([]jsonpatchv2.JsonPatchOperation, error) {
	if interrupter := p.getInterrupter(obj); interrupter != nil {
		return interrupter.OnMutating(obj, oldObj)
	}

	return nil, nil
}

func (p *policyInterrupterImpl) OnValidating(obj, oldObj *unstructured.Unstructured) error {
	if interrupter := p.getInterrupter(obj); interrupter != nil {
		return interrupter.OnValidating(obj, oldObj)
	}

	return nil
}

func (p *policyInterrupterImpl) isKnownPolicy(obj *unstructured.Unstructured) bool {
	group := strings.Split(obj.GetAPIVersion(), "/")[0]
	return group != policyv1alpha1.SchemeGroupVersion.Group
}

func (p *policyInterrupterImpl) getInterrupter(obj *unstructured.Unstructured) PolicyInterrupter {
	if !p.isKnownPolicy(obj) {
		return nil
	}

	// check crd type before call this func
	switch obj.GetKind() {
	case "ClusterValidatePolicy":
		return p.clusterValidatePolicyInterrupter
	case "ClusterOverridePolicy":
		return p.clusterOverridePolicyInterrupter
	case "OverridePolicy":
		return p.overridePolicyInterrupter
	}

	return nil
}

func NewPolicyInterrupter(op, cop, cvp PolicyInterrupter) PolicyInterrupter {
	return &policyInterrupterImpl{
		overridePolicyInterrupter:        op,
		clusterOverridePolicyInterrupter: cop,
		clusterValidatePolicyInterrupter: cvp,
	}
}
