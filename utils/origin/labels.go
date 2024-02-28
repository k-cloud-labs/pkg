package origin

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
)

type Labels struct {
	Value map[string]string
}

func (l *Labels) GetJsonPatch(rawObj *unstructured.Unstructured, Replace bool, operator policyv1alpha1.OverriderOperator) (*OverrideOption, error) {
	currentLabels := rawObj.GetLabels()
	newLabels := l.Value

	op := &OverrideOption{
		Op:    string(policyv1alpha1.OverriderOpReplace),
		Path:  "/metadata/labels",
		Value: newLabels,
	}

	if Replace {
		return op, nil
	}

	switch operator {
	case policyv1alpha1.OverriderOpAdd, policyv1alpha1.OverriderOpReplace:
		if currentLabels == nil {
			currentLabels = newLabels
		} else {
			for key, value := range newLabels {
				currentLabels[key] = value
			}
		}

	case policyv1alpha1.OverriderOpRemove:
		if currentLabels != nil {
			for key := range newLabels {
				delete(currentLabels, key)
			}
		}
	}

	op.Value = currentLabels
	return op, nil
}
