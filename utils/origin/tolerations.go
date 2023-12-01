package origin

import (
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/utils/util"
)

type Tolerations struct {
	Value []corev1.Toleration
}

func (t *Tolerations) GetJsonPatch(rawObj *unstructured.Unstructured, Replace bool, operator policyv1alpha1.OverriderOperator) (*OverrideOption, error) {
	k := rawObj.GetKind()
	currentTolerations := []corev1.Toleration{}
	op := &OverrideOption{
		Op: string(policyv1alpha1.OverriderOpReplace),
	}

	if k == PodKind {
		op.Path = "/spec/tolerations"
		pod, err := util.ConvertToPod(rawObj)
		if err != nil {
			return nil, fmt.Errorf("get spec.tolerations error: %s", err.Error())
		}

		if pod != nil {
			currentTolerations = pod.Spec.Tolerations
		}

	} else if k == DeploymentKind {
		op.Path = "/spec/template/spec/tolerations"
		deploy, err := util.ConvertToDeployment(rawObj)
		if err != nil {
			return nil, fmt.Errorf("get spec.template.spec.tolerations error: %s", err.Error())
		}
		if deploy != nil {
			currentTolerations = deploy.Spec.Template.Spec.Tolerations
		}

	} else {
		return nil, errors.New("unsupported kind type error")
	}

	if Replace || ((operator == policyv1alpha1.OverriderOpAdd || operator == policyv1alpha1.OverriderOpReplace) && len(currentTolerations) == 0) {
		op.Value = t.Value
		return op, nil
	}

	if operator == policyv1alpha1.OverriderOpRemove && len(currentTolerations) == 0 {
		return op, nil
	}

	currentTolerationsStringMap := map[string]bool{}
	for i := range currentTolerations {
		currentTolerationsStringMap[tolerationToString(currentTolerations[i])] = true
	}

	var newTolerations []corev1.Toleration
	switch operator {
	case policyv1alpha1.OverriderOpAdd, policyv1alpha1.OverriderOpReplace:
		inNeedTolerationsStringMap := map[string]bool{}
		for i := range t.Value {
			var newToleration corev1.Toleration
			tolerationString := tolerationToString(t.Value[i])
			if currentTolerationsStringMap[tolerationString] {
				inNeedTolerationsStringMap[tolerationString] = true
				newToleration = corev1.Toleration{
					Key:               t.Value[i].Key,
					Value:             t.Value[i].Value,
					Operator:          t.Value[i].Operator,
					Effect:            t.Value[i].Effect,
					TolerationSeconds: t.Value[i].TolerationSeconds,
				}
				newTolerations = append(newTolerations, newToleration)
				continue
			}
			newTolerations = append(newTolerations, t.Value[i])
		}

		for i := range currentTolerations {
			tolerationString := tolerationToString(currentTolerations[i])
			if !inNeedTolerationsStringMap[tolerationString] {
				newTolerations = append(newTolerations, currentTolerations[i])
			}
		}

	case policyv1alpha1.OverriderOpRemove:
		needDeleteTolerationsStringMap := map[string]bool{}
		for i := range t.Value {
			tolerationString := tolerationToString(t.Value[i])
			if currentTolerationsStringMap[tolerationString] {
				needDeleteTolerationsStringMap[tolerationString] = true
				continue
			}
		}

		for i := range currentTolerations {
			tolerationString := tolerationToString(currentTolerations[i])
			if !needDeleteTolerationsStringMap[tolerationString] {
				newTolerations = append(newTolerations, currentTolerations[i])
			}
		}
	}

	op.Value = newTolerations
	return op, nil
}

func tolerationToString(this corev1.Toleration) string {
	s := strings.Join([]string{`&Toleration{`,
		`Key:` + fmt.Sprintf("%v", this.Key) + `,`,
		`Operator:` + fmt.Sprintf("%v", this.Operator) + `,`,
		`Value:` + fmt.Sprintf("%v", this.Value) + `,`,
		`Effect:` + fmt.Sprintf("%v", this.Effect) + `,`,
		`}`,
	}, "")
	return s
}
