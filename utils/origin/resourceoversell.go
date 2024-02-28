package origin

import (
	"errors"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/utils/util"
)

type ResourceOversell struct {
	Value map[policyv1alpha1.ResourceType]map[string]policyv1alpha1.Float64
	Count int
}

func (r *ResourceOversell) GetJsonPatch(rawObj *unstructured.Unstructured, Replace bool, operator policyv1alpha1.OverriderOperator) (*OverrideOption, error) {
	if operator == policyv1alpha1.OverriderOpAdd || operator == policyv1alpha1.OverriderOpRemove {
		return nil, errors.New("unsupported operator type error")
	}

	var currentContainers []corev1.Container
	k := rawObj.GetKind()

	op := &OverrideOption{
		Op: string(policyv1alpha1.OverriderOpReplace),
	}

	if k == PodKind {
		op.Path = "/spec/containers/" + strconv.Itoa(r.Count) + "/resources"
		pod, err := util.ConvertToPod(rawObj)
		if err != nil {
			return nil, fmt.Errorf("get spec.containers resources error: %s", err.Error())
		}

		if pod != nil {
			currentContainers = pod.Spec.Containers
		}

		if len(currentContainers) == 0 {
			return nil, fmt.Errorf("containers not found in pod spec")
		}
	} else if k == DeploymentKind {
		op.Path = "/spec/template/spec/containers/" + strconv.Itoa(r.Count) + "/resources"
		deploy, err := util.ConvertToDeployment(rawObj)
		if err != nil {
			return nil, fmt.Errorf("get spec.template.spec.containers resources error: %s", err.Error())
		}

		if deploy != nil {
			currentContainers = deploy.Spec.Template.Spec.Containers
		}

		if len(currentContainers) == 0 {
			return nil, fmt.Errorf("containers not found in deployment template pod spec")
		}
	} else {
		return nil, errors.New("unsupported kind type error")
	}

	if len(currentContainers) <= r.Count {
		return nil, errors.New("containerCount cannot be greater than the number of containers in the pod")
	}

	currentResourceList := currentContainers[r.Count].Resources
	resultResourceList := currentResourceList

	if r.Value[policyv1alpha1.RequestResourceType] != nil {
		for k, v := range r.Value[policyv1alpha1.RequestResourceType] {
			switch corev1.ResourceName(k) {
			case corev1.ResourceCPU:
				q := currentResourceList.Requests[corev1.ResourceName(k)]
				if q.MilliValue() == 0 {
					continue
				}
				overSellCpuValue := float64(q.MilliValue()) * (*v.ToFloat64())
				overSellCpuQuantity := resource.NewMilliQuantity(int64(overSellCpuValue), resource.DecimalSI)
				resultResourceList.Requests[corev1.ResourceName(k)] = *overSellCpuQuantity
			case corev1.ResourceMemory, corev1.ResourceStorage, corev1.ResourceEphemeralStorage:
				q := currentResourceList.Requests[corev1.ResourceName(k)]
				if q.Value() == 0 {
					continue
				}
				overSellStorageValue := float64(q.Value()) * (*v.ToFloat64())
				overSellStorageQuantity := resource.NewQuantity(int64(overSellStorageValue), resource.BinarySI)
				resultResourceList.Requests[corev1.ResourceName(k)] = *overSellStorageQuantity
			}
		}
	}

	if r.Value[policyv1alpha1.LimitResourceType] != nil {
		for k, v := range r.Value[policyv1alpha1.LimitResourceType] {
			switch corev1.ResourceName(k) {
			case corev1.ResourceCPU:
				q := currentResourceList.Limits[corev1.ResourceName(k)]
				if q.MilliValue() == 0 {
					continue
				}
				overSellCpuValue := float64(q.MilliValue()) * (*v.ToFloat64())
				overSellCpuQuantity := resource.NewMilliQuantity(int64(overSellCpuValue), resource.DecimalSI)
				resultResourceList.Limits[corev1.ResourceName(k)] = *overSellCpuQuantity
			case corev1.ResourceMemory, corev1.ResourceStorage, corev1.ResourceEphemeralStorage:
				q := currentResourceList.Limits[corev1.ResourceName(k)]
				if q.Value() == 0 {
					continue
				}
				overSellStorageValue := float64(q.Value()) * (*v.ToFloat64())
				overSellStorageQuantity := resource.NewQuantity(int64(overSellStorageValue), resource.BinarySI)
				resultResourceList.Limits[corev1.ResourceName(k)] = *overSellStorageQuantity
			}
		}
	}

	op.Value = resultResourceList
	return op, nil
}
