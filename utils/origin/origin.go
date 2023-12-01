package origin

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
)

type OriginValue interface {
	GetJsonPatch(rawObj *unstructured.Unstructured, Replace bool, operator v1alpha1.OverriderOperator) (*OverrideOption, error)
}

type OverrideOption struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
}

const (
	DeploymentKind string = "Deployment"
	PodKind        string = "Pod"
)
