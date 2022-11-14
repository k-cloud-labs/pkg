package model

import (
	"k8s.io/apimachinery/pkg/util/intstr"
)

type PodAvailableBadgeRenderData struct {
	MaxUnavailable *intstr.IntOrString
	MinAvailable   *intstr.IntOrString
	OwnerReference *ResourceRefer
}
