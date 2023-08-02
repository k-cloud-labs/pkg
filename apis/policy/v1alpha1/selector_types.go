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
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type FieldSelector struct {
	// matchFields is a map of {key,value} pairs. A single {key,value} in the matchFields
	// map is equivalent to an element of matchExpressions, whose key field is "key", the
	// operator is "In", and the values array contains only "value".
	// +optional
	MatchFields map[string]string `json:"matchFields,omitempty"`
	// matchExpressions is a list of fields selector requirements. The requirements are ANDed.
	// +optional
	MatchExpressions []*FieldSelectorRequirement `json:"matchExpressions,omitempty"`
}

type FieldSelectorRequirement struct {
	// Field is the field key that the selector applies to.
	// Must provide whole path of key, such as `metadata.annotations.uid`
	Field string `json:"field"`
	// operator represents a key's relationship to a set of values.
	// Valid operators are In, NotIn, Exists and DoesNotExist.
	Operator metav1.LabelSelectorOperator `json:"operator"`
	// values is an array of string values. If the operator is In or NotIn,
	// the values array must be non-empty. If the operator is Exists or DoesNotExist,
	// the values array must be empty.
	// +optional
	Value []string `json:"value,omitempty"`
}

func (r *FieldSelectorRequirement) MatchObject(obj *unstructured.Unstructured) (bool, error) {
	v, found, err := fetchObjValue(obj, r.Field)
	if err != nil {
		return false, err
	}

	switch r.Operator {
	case metav1.LabelSelectorOpExists:
		return found, nil
	case metav1.LabelSelectorOpDoesNotExist:
		return !found, nil
	case metav1.LabelSelectorOpIn:
		if !found {
			return found, nil
		}

		for _, s := range r.Value {
			if s == v {
				return true, nil
			}
		}

		return false, nil
	case metav1.LabelSelectorOpNotIn:
		var in bool
		for _, s := range r.Value {
			if s == v {
				in = true
				break
			}
		}

		return !in, nil
	default:
		return false, fmt.Errorf("unknown operator:%v", r.Operator)
	}
}

func (f *FieldSelector) MatchObject(obj *unstructured.Unstructured) (bool, error) {
	for k, v := range f.MatchFields {
		match, err := matchObj(obj, k, v)
		if err != nil {
			return false, err
		}

		if !match {
			return match, nil
		}
	}

	for i := range f.MatchExpressions {
		match, err := f.MatchExpressions[i].MatchObject(obj)
		if err != nil {
			return false, err
		}

		if !match {
			return match, nil
		}
	}

	return true, nil
}

func matchObj(obj *unstructured.Unstructured, field, value string) (bool, error) {
	v, found, err := fetchObjValue(obj, field)
	if err != nil {
		return false, err
	}

	return found && value == v, nil
}

func fetchObjValue(obj *unstructured.Unstructured, field string) (string, bool, error) {
	if index := strings.Index(field, ".annotations."); index != -1 {
		return unstructured.NestedString(obj.Object, append(strings.Split(field[0:index], "."), "annotations", field[index+13:])...)
	} else {
		return unstructured.NestedString(obj.Object, strings.Split(field, ".")...)
	}
}
