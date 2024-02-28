package origin

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"errors"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
)

type NodeSelector struct {
	Value map[string]string
}

func (a *NodeSelector) GetJsonPatch(rawObj *unstructured.Unstructured, Replace bool, operator policyv1alpha1.OverriderOperator) (*OverrideOption, error) {
	op := &OverrideOption{
		Op: string(policyv1alpha1.OverriderOpReplace),
	}

	k := rawObj.GetKind()
	var nodeSelectorField map[string]string
	var err error

	if k == PodKind {
		nodeSelectorField, _, err = unstructured.NestedStringMap(rawObj.Object, "spec", "nodeSelector")
		if err != nil {
			return nil, fmt.Errorf("get spec.nodeSelector error: %s", err.Error())
		}
		op.Path = "/spec/nodeSelector"
	} else if k == DeploymentKind {
		nodeSelectorField, _, err = unstructured.NestedStringMap(rawObj.Object, "spec", "template", "spec", "nodeSelector")
		if err != nil {
			return nil, fmt.Errorf("get spec.template.spec.nodeSelector error: %s", err.Error())
		}
		op.Path = "/spec/template/spec/nodeSelector"
	} else {
		return nil, errors.New("unsupported kind type error")
	}

	currentNodeSelector := nodeSelectorField
	newNodeSelector := a.Value

	if Replace {
		op.Value = newNodeSelector
		return op, nil
	}

	switch operator {
	case policyv1alpha1.OverriderOpAdd, policyv1alpha1.OverriderOpReplace:
		if currentNodeSelector == nil {
			currentNodeSelector = newNodeSelector
		} else {
			for key, value := range newNodeSelector {
				currentNodeSelector[key] = value
			}
		}

	case policyv1alpha1.OverriderOpRemove:
		if currentNodeSelector != nil {
			for key := range newNodeSelector {
				delete(currentNodeSelector, key)
			}
		}
	}

	op.Value = currentNodeSelector
	return op, nil
}
