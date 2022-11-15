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
	"strconv"

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

	// Template of rule which defines override rule, and
	// it will be rendered to CUE and store in RenderedCue field, so
	//if there are any data added manually will be erased.
	// +optional
	Template *OverrideRuleTemplate `json:"template,omitempty"`

	// RenderedCue represents override rule defined by Template.
	// Don't modify the value of this field, modify Rules instead of.
	// +optional
	RenderedCue string `json:"renderedCue,omitempty"`
}

// OverrideRuleType is definition for type of single override rule template
// +kubebuilder:validation:Enum=annotations;labels;resourcesOversell;resources;affinity;tolerations
type OverrideRuleType string

// The valid RuleTypes
const (
	// OverrideRuleTypeAnnotations - `annotations`
	OverrideRuleTypeAnnotations OverrideRuleType = "annotations"
	// OverrideRuleTypeLabels - `labels`
	OverrideRuleTypeLabels OverrideRuleType = "labels"
	// OverrideRuleTypeResourcesOversell - `resourcesOversell`
	OverrideRuleTypeResourcesOversell OverrideRuleType = "resourcesOversell"
	// OverrideRuleTypeResources - `resources`
	OverrideRuleTypeResources OverrideRuleType = "resources"
	// OverrideRuleTypeAffinity - `affinity`
	OverrideRuleTypeAffinity OverrideRuleType = "affinity"
	// OverrideRuleTypeTolerations - `tolerations`
	OverrideRuleTypeTolerations OverrideRuleType = "tolerations"
)

// ValueType defines whether value is specified by user or refer from other object
// +kubebuilder:validation:Enum=const;ref
type ValueType string

const (
	// ValueTypeConst means value is specified exactly.
	ValueTypeConst ValueType = "const"
	// ValueTypeRefer means value is refer from other object
	ValueTypeRefer ValueType = "ref"
)

// ValueRefFrom defines where the override value comes from when value is refer other object or http response
// +kubebuilder:validation:Enum=current;old;k8s;owner;http
type ValueRefFrom string

// Valid ValueRefFrom
const (
	// FromCurrentObject means read data from current k8s object(the newest one when update operate intercept)
	FromCurrentObject ValueRefFrom = "current"
	// FromOldObject means read data from old object, only used when object be updated
	FromOldObject ValueRefFrom = "old"
	// FromK8s - read data from other object in current kubernetes
	FromK8s ValueRefFrom = "k8s"
	// FromOwnerReference - load first owner reference from current object
	FromOwnerReference = "owner"
	// FromHTTP - read data from http response
	FromHTTP ValueRefFrom = "http"
)

// OverrideRuleTemplate represents a single template of rule definition
type OverrideRuleTemplate struct {
	// Type represents current rule operate field type.
	// +kubebuilder:validation:Enum=annotations;labels;resources;resourcesOversell;tolerations;affinity
	// +required
	Type OverrideRuleType `json:"type,omitempty"`
	// Operation represents current operation type.
	// +kubebuilder:validation:Enum=add;replace;remove
	// +required
	Operation OverriderOperator `json:"operation,omitempty"`
	// Path is field path of current object(e.g. `/spec/affinity`)
	// If current type is annotations or labels, then only need to provide the key, no need whole path.
	// +optional
	Path string `json:"path,omitempty"`
	// Value sets exact value for rule, like enum or numbers
	// Must set value when type is regex.
	// +optional
	Value *ConstantValue `json:"value,omitempty"`
	// ValueRef represents for value reference from current or remote object.
	// Need specify the type of object and how to get it.
	// +optional
	ValueRef *ResourceRefer `json:"valueRef,omitempty"`
	// Resources valid only when the type is `resources`
	// +optional
	Resources *v1.ResourceRequirements `json:"resources,omitempty"`
	// ResourcesOversell valid only when the type is `resourcesOversell`
	// +optional
	ResourcesOversell *ResourcesOversellRule `json:"resourcesOversell,omitempty"`
	// Tolerations valid only when the type is `tolerations`
	// +optional
	Tolerations []*v1.Toleration `json:"tolerations,omitempty"`
	// Affinity valid only when the type is `affinity`
	// +optional
	Affinity *v1.Affinity `json:"affinity,omitempty"`
}

// ResourceRefer defines different types of ref data
type ResourceRefer struct {
	// From represents where this referenced object are.
	// +kubebuilder:validation:Enum=current;old;k8s;owner;http
	// +required
	From ValueRefFrom `json:"from,omitempty"`
	// Path has different meaning, it represents current object field path like "/spec/replica" when From equals "current"
	// and it also can be format like "data.result.x.y" when From equals "http", it represents the path in http response
	// Only when From is owner(means refer current object owner), the path can be empty.
	// +optional
	Path string `json:"path,omitempty"`
	// K8s means refer another object from current cluster.
	// +optional
	K8s *ResourceSelector `json:"k8s,omitempty"`
	// Http means refer data from remote api.
	// +optional
	Http *HttpDataRef `json:"http,omitempty"`
}

// HttpDataRef defines a http request essential params
type HttpDataRef struct {
	// URL as whole http url
	// +required
	URL string `json:"url,omitempty"`
	// Method as basic http method(e.g. GET or POST)
	// +required
	Method string `json:"method,omitempty"`
	// Header represents the custom header added to http request header.
	// +optional
	Header map[string]string `json:"header,omitempty"`
	// Params represents the query value for http request.
	// +optional
	Params map[string]string `json:"params,omitempty"`
	// Body represents the json body when http method is POST.
	Body apiextensionsv1.JSON `json:"body,omitempty"`
}

// ResourcesOversellRule defines factor of resource oversell
type ResourcesOversellRule struct {
	// CpuFactor factor of cup oversell, it is float number less than 1, the range of value is (0,1.0)
	// +optional
	CpuFactor Float64 `json:"cpuFactor,omitempty"`
	// MemoryFactor factor of cup oversell, it is float number less than 1, the range of value is (0,1.0)
	// +optional
	MemoryFactor Float64 `json:"memoryFactor,omitempty"`
	// DiskFactor factor of cup oversell, it is float number less than 1, the range of value is (0,1.0)
	// +optional
	DiskFactor Float64 `json:"diskFactor,omitempty"`
}

// Float64 is alias for float64 as string
type Float64 string

func (f Float64) ValidFactor() bool {
	f64, err := strconv.ParseFloat(string(f), 64)
	if err != nil {
		return false
	}

	return f64 > 0 && f64 < 1
}

// ToFloat64 converts string to pointer to float64 and return nil if convert got error
func (f Float64) ToFloat64() *float64 {
	f64, err := strconv.ParseFloat(string(f), 64)
	if err != nil {
		return nil
	}

	return &f64
}

// ConstantValue defines exact types. Only one of field can be set.
type ConstantValue struct {
	// String as a string
	// +optional
	String *string `json:"string,omitempty"`
	// Integer as an integer(int64)
	// +optional
	Integer *int64 `json:"integer,omitempty"`
	// Float as float but use string to store, so please provide in comma (e.g. float: "1.2")
	// +optional
	Float *Float64 `json:"float,omitempty"`
	// Boolean only true or false can be recognized.
	// +optional
	Boolean *bool `json:"boolean,omitempty"`
	// StringSlice as a slice of string(e.g. ["a","b"])
	// +optional
	StringSlice []string `json:"stringSlice,omitempty"`
	// IntegerSlice as a slice of integer(int64) (e.g. [1,2,3])
	// +optional
	IntegerSlice []int64 `json:"integerSlice,omitempty"`
	// FloatSlice as a slice of float but using string (e.g. ["1.2", "2.3"])
	// +optional
	FloatSlice []Float64 `json:"floatSlice,omitempty"`
	// StringMap as key-value set and both are string.
	// +optional
	StringMap map[string]string `json:"stringMap,omitempty"`
}

// Value return first non-nil value, it returns nil if all fields are empty.
func (t *ConstantValue) Value() any {
	if t == nil {
		return nil
	}

	if t.String != nil {
		return *t.String
	}

	if t.Integer != nil {
		return *t.Integer
	}

	if t.Float != nil {
		return *t.Float
	}

	if t.Boolean != nil {
		return *t.Boolean
	}

	if len(t.StringSlice) > 0 {
		return t.StringSlice
	}

	if len(t.IntegerSlice) > 0 {
		return t.IntegerSlice
	}

	if len(t.FloatSlice) > 0 {
		return t.FloatSlice
	}

	if len(t.StringMap) > 0 {
		return t.StringMap
	}

	return nil
}

func (t *ConstantValue) GetSlice() []any {
	if t == nil {
		return nil
	}

	var result []any
	if len(t.StringSlice) > 0 {
		for _, s := range t.StringSlice {
			result = append(result, s)
		}

		return result
	}

	if len(t.IntegerSlice) > 0 {
		for _, s := range t.IntegerSlice {
			result = append(result, s)
		}

		return result
	}

	if len(t.FloatSlice) > 0 {
		for _, s := range t.FloatSlice {
			result = append(result, s)
		}

		return result
	}

	return nil
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
	// OverriderOpAdd - "add" value to object
	OverriderOpAdd OverriderOperator = "add"
	// OverriderOpRemove - remove field form object
	OverriderOpRemove OverriderOperator = "remove"
	// OverriderOpReplace - remove and add value(if specified path doesn't exist, it will add directly)
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
