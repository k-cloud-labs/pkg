/*
Copyright 2022 by k-cloud-labs org.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OverridePolicySpec defines the desired behavior of OverridePolicy.
type OverridePolicySpec struct {
	// ResourceSelectors restricts resource types that this override policy applies to.
	// nil means matching all resources.
	// +optional
	ResourceSelectors []ResourceSelector `json:"resourceSelectors,omitempty"`

	// OverrideRules defines a collection of override rules on target operations.
	// +required
	OverrideRules []RuleWithOperation `json:"overrideRules"`
}

// RuleWithOperation defines the override rules on operations.
type RuleWithOperation struct {
	// TargetOperations is the operations the admission hook cares about - CREATE, UPDATE, DELETE, CONNECT or *
	// for all of those operations and any future admission operations that are added.
	// If '*' is present, the length of the slice must be one.
	// Required.
	TargetOperations []admissionv1.Operation `json:"targetOperations,omitempty"`

	// Overriders represents the override rules that would apply on resources
	// +required
	Overriders Overriders `json:"overriders"`
}

// ResourceSelector the resources will be selected.
type ResourceSelector struct {
	// APIVersion represents the API version of the target resources.
	// +required
	APIVersion string `json:"apiVersion"`

	// Kind represents the Kind of the target resources.
	// +required
	Kind string `json:"kind"`

	// Namespace of the target resource.
	// Default is empty, which means inherit from the parent object scope.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name of the target resource.
	// Default is empty, which means selecting all resources.
	// +optional
	Name string `json:"name,omitempty"`

	// A label query over a set of resources.
	// If name is not empty, labelSelector will be ignored.
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}

// Overriders offers various alternatives to represent the override rules.
//
// If more than one alternative exist, they will be applied with following order:
// - RenderCue
// - Cue
// - Plaintext
type Overriders struct {
	// Plaintext represents override rules defined with plaintext overriders.
	// +optional
	Plaintext []PlaintextOverrider `json:"plaintext,omitempty"`

	// Cue represents override rules defined with cue code.
	// +optional
	Cue string `json:"cue,omitempty"`

	// Rules is a list of rule which defines override rule, and
	// it will be rendered to CUE and store in RenderedCue field, so
	//if there are any data added manually will be erased.
	// +optional
	Rules []Rule `json:"rules,omitempty"`

	// RenderedCue represents override rules defined by Rules.
	// Don't modify the value of this field, modify Rules instead of.
	// +optional
	RenderedCue string `json:"renderedCue,omitempty"`
}

// RuleType is definition for type of single rule
type RuleType string

// The valid RuleTypes
const (
	RuleTypeAnnotations      RuleType = "annotations"
	RuleTypeLabels           RuleType = "labels"
	RuleTypeResourceOversell RuleType = "resourceOversell"
	RuleTypeResource         RuleType = "resource"
	RuleTypeAffinity         RuleType = "affinity"
	RuleTypeTolerations      RuleType = "tolerations"
)

// Operation means override operation, like add/update value or delete fields
type Operation string

// Valid Operations
const (
	OperationAdd     Operation = "add"
	OperationReplace Operation = "replace"
	OperationDelete  Operation = "delete"
)

// ValueRefFrom defines where the override value comes from when value is refer other object or http response
type ValueRefFrom string

// Valid ValueRefFrom
const (
	// FromOwn means read data from own object
	FromOwn ValueRefFrom = "own"
	// FromK8s - read data from other object in current kubernetes
	FromK8s ValueRefFrom = "k8s"
	// FromHTTP - read data from http response
	FromHTTP ValueRefFrom = "http"
)

// Rule represents a single rule definition
type Rule struct {
	Type      RuleType  `json:"type,omitempty"`
	Operation Operation `json:"operation,omitempty"`
	Path      string    `json:"path,omitempty"`
	Value     any       `json:"value,omitempty"`
	ValueRef  *ValueRef `json:"valueRef,omitempty"`

	//resource
	Resource *v1.ResourceRequirements `json:"resource,omitempty"`
	// resource oversell
	ResourceOversell *ResourceOversellRule `json:"resourceOversell,omitempty"`
	// toleration
	Tolerations []*v1.Toleration `json:"tolerations,omitempty"`
	// affinity
	Affinity *v1.Affinity `json:"affinity,omitempty"`
}

// ValueRef defines different types of ref data
type ValueRef struct {
	From ValueRefFrom `json:"from"`
	// Path has different meaning, it represents current object field path like "/spec/replica" when From equals "own"
	// and it also can be format like "data.result.x.y" when From equals "http", it represents the path in http response
	Path string `json:"path"`
	// ref k8s resource
	*ResourceSelector `json:",inline"`
	// ref http response
	*HttpDataRef `json:",inline"`
}

// HttpDataRef defines a http request essential params
type HttpDataRef struct {
	URL    string            `json:"url,omitempty"`
	Method string            `json:"method,omitempty"`
	Params map[string]string `json:"params,omitempty"`
}

// ResourceOversellRule defines factor of resource oversell
type ResourceOversellRule struct {
	CpuFactor    float64 `json:"cpuFactor,omitempty"`
	MemoryFactor float64 `json:"memoryFactor,omitempty"`
	DiskFactor   float64 `json:"diskFactor,omitempty"`
}

// PlaintextOverrider is a simple overrider that overrides target fields
// according to path, operator and value.
type PlaintextOverrider struct {
	// Path indicates the path of target field
	Path string `json:"path"`
	// Operator indicates the operation on target field.
	// Available operators are: add, update and remove.
	// +kubebuilder:validation:Enum=add;remove;replace
	Operator OverriderOperator `json:"op"`
	// Value to be applied to target field.
	// Must be empty when operator is Remove.
	// +optional
	Value apiextensionsv1.JSON `json:"value,omitempty"`
}

// OverriderOperator is the set of operators that can be used in an overrider.
type OverriderOperator string

// These are valid overrider operators.
const (
	OverriderOpAdd     OverriderOperator = "add"
	OverriderOpRemove  OverriderOperator = "remove"
	OverriderOpReplace OverriderOperator = "replace"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:shortName=op

// OverridePolicy represents the policy that overrides a group of resources.
type OverridePolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec OverridePolicySpec `json:"spec,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OverridePolicyList contains a list of OverridePolicy
type OverridePolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OverridePolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OverridePolicy{}, &OverridePolicyList{})
}
