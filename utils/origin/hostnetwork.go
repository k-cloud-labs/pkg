package origin

import (
	"errors"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
)

type HostNetWork struct {
	Value bool
}

func (h *HostNetWork) GetJsonPatch(rawObj *unstructured.Unstructured, Replace bool, operator policyv1alpha1.OverriderOperator) (*OverrideOption, error) {
	if operator == policyv1alpha1.OverriderOpAdd || operator == policyv1alpha1.OverriderOpRemove {
		return nil, errors.New("unsupported operator type error")
	}

	k := rawObj.GetKind()
	newHostNetwork := h.Value

	op := &OverrideOption{
		Value: newHostNetwork,
		Op:    string(policyv1alpha1.OverriderOpReplace),
	}

	if k == PodKind {
		op.Path = "/spec/hostNetwork"
	} else if k == DeploymentKind {
		op.Path = "/spec/template/spec/hostNetwork"
	} else {
		return nil, errors.New("unsupported kind type error")
	}

	return op, nil
}
