package origin

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
)

type Annotations struct {
	Value map[string]string
}

func (a *Annotations) GetJsonPatch(rawObj *unstructured.Unstructured, Replace bool, operator policyv1alpha1.OverriderOperator) (*OverrideOption, error) {
	currentAnnotations := rawObj.GetAnnotations()
	newAnnotations := a.Value

	op := &OverrideOption{
		Op:    string(policyv1alpha1.OverriderOpReplace),
		Path:  "/metadata/annotations",
		Value: newAnnotations,
	}

	if Replace {
		return op, nil
	}

	switch operator {
	case policyv1alpha1.OverriderOpAdd, policyv1alpha1.OverriderOpReplace:
		if currentAnnotations == nil {
			currentAnnotations = newAnnotations
		} else {
			for key, value := range newAnnotations {
				currentAnnotations[key] = value
			}
		}

	case policyv1alpha1.OverriderOpRemove:
		if currentAnnotations != nil {
			for key := range newAnnotations {
				delete(currentAnnotations, key)
			}
		}
	}

	op.Value = currentAnnotations
	return op, nil
}
