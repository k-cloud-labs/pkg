package origin

import (
	"errors"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
)

type SchedulerName struct {
	Value string
}

func (s *SchedulerName) GetJsonPatch(rawObj *unstructured.Unstructured, Replace bool, operator policyv1alpha1.OverriderOperator) (*OverrideOption, error) {
	if operator == policyv1alpha1.OverriderOpAdd || operator == policyv1alpha1.OverriderOpRemove {
		return nil, errors.New("unsupported operator type error")
	}

	k := rawObj.GetKind()
	newSchedulerName := s.Value

	op := &OverrideOption{
		Value: newSchedulerName,
		Op:    string(policyv1alpha1.OverriderOpReplace),
	}

	if k == PodKind {
		op.Path = "/spec/schedulerName"
	} else if k == DeploymentKind {
		op.Path = "/spec/template/spec/schedulerName"
	} else {
		return nil, errors.New("unsupported kind type error")
	}

	return op, nil
}
