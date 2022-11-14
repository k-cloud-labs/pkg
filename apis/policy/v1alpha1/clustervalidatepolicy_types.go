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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterValidatePolicySpec defines the desired behavior of ClusterValidatePolicy.
type ClusterValidatePolicySpec struct {
	// ResourceSelectors restricts resource types that this validate policy applies to.
	// nil means matching all resources.
	// +optional
	ResourceSelectors []ResourceSelector `json:"resourceSelectors,omitempty"`

	// ValidateRules defines a collection of validate rules on target operations.
	// +required
	ValidateRules []ValidateRuleWithOperation `json:"validateRules"`
}

// ValidateRuleWithOperation defines the validate rules on operations.
type ValidateRuleWithOperation struct {
	// Operations is the operations the admission hook cares about - CREATE, UPDATE, DELETE, CONNECT or *
	// for all of those operations and any future admission operations that are added.
	// If '*' is present, the length of the slice must be one.
	// Required.
	TargetOperations []admissionv1.Operation `json:"targetOperations,omitempty"`

	// Cue represents validate rules defined with cue code.
	// +optional
	Cue string `json:"cue"`

	// Template of condition which defines validate cond, and
	// it will be rendered to CUE and store in RenderedCue field, so
	// if there are any data added manually will be erased.
	// +optional
	Template *TemplateCondition `json:"template,omitempty"`

	// RenderedCue represents validate rule defined by Template.
	// Don't modify the value of this field, modify Rules instead of.
	// +optional
	RenderedCue string `json:"renderedCue,omitempty"`
}

// Cond is validation condition for validator
// +kubebuilder:validation:Enum=Equal;NotEqual;Exist;NotExist;In;NotIn;Gt;Gte;Lt;Lte
type Cond string

const (
	// CondEqual - `Equal`
	CondEqual Cond = "Equal"
	// CondNotEqual - `NotEqual`
	CondNotEqual Cond = "NotEqual"
	// CondExist - `Exist`
	CondExist Cond = "Exist"
	// CondNotExist - `NotExist`
	CondNotExist Cond = "NotExist"
	// CondIn - `In`
	CondIn Cond = "In"
	// CondNotIn - `NotIn`
	CondNotIn Cond = "NotIn"
	// CondGreater - `Gt`
	CondGreater Cond = "Gt"
	// CondGreaterOrEqual - `Gte`
	CondGreaterOrEqual Cond = "Gte"
	// CondLesser - `Lt`
	CondLesser Cond = "Lt"
	// CondLesserOrEqual - `Lte`
	CondLesserOrEqual Cond = "Lte"
)

type TemplateCondition struct {
	// Cond represents type of condition (e.g. Equal, Exist)
	// +kubebuilder:validation:Enum=Equal;NotEqual;Exist;NotExist;In;NotIn;Gt;Gte;Lt;Lte
	// +required
	Cond Cond `json:"cond,omitempty"`
	// DataRef represents for data reference from current or remote object.
	// Need specify the type of object and how to get it.
	// +required
	DataRef *ResourceRefer `json:"dataRef,omitempty"`
	// Message specify reject message when policy hit.
	// +required
	Message string `json:"message,omitempty"`
	// Value sets exact value for rule, like enum or numbers
	// +optional
	Value *CustomTypes `json:"value,omitempty"`
	// ValueRef represents for value reference from current or remote object.
	// Need specify the type of object and how to get it.
	// +optional
	ValueRef *ResourceRefer `json:"valueRef,omitempty"`
}

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:scope="Cluster",shortName=cvp
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterValidatePolicy represents the cluster-wide policy that validate a group of resources.
type ClusterValidatePolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterValidatePolicySpec `json:"spec,omitempty"`
}

// +kubebuilder:resource:scope="Cluster"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterValidatePolicyList contains a list of ClusterValidatePolicy
type ClusterValidatePolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterValidatePolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterValidatePolicy{}, &ClusterValidatePolicyList{})
}
