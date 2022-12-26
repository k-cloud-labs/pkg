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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/selection"
)

type FieldSelector struct {
	MatchFields []*MatchField `json:"matchFields,omitempty"`
}

type MatchField struct {
	Field string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

var (
	_ fields.Selector = &FieldSelector{}
	_ fields.Selector = &MatchField{}
)

func (m *MatchField) Matches(field fields.Fields) bool {
	return field.Get(m.Field) == m.Value
}

func (m *MatchField) Empty() bool {
	return false
}

func (m *MatchField) RequiresExactMatch(field string) (value string, found bool) {
	if m.Field == field {
		return m.Value, true
	}

	return "", false
}

func (m *MatchField) Transform(fn fields.TransformFunc) (fields.Selector, error) {
	field, value, err := fn(m.Field, m.Value)
	if err != nil {
		return nil, err
	}
	if len(field) == 0 && len(value) == 0 {
		return fields.Everything(), nil
	}

	return &MatchField{field, value}, nil
}

func (m *MatchField) Requirements() fields.Requirements {
	return []fields.Requirement{{
		Field:    m.Field,
		Operator: selection.Equals,
		Value:    m.Value,
	}}
}

func (m *MatchField) String() string {
	return fmt.Sprintf("%v=%v", m.Field, fields.EscapeValue(m.Value))
}

func (m *MatchField) DeepCopySelector() fields.Selector {
	if m == nil {
		return nil
	}
	out := new(MatchField)
	*out = *m
	return out
}

func (m *MatchField) MatchObject(obj *unstructured.Unstructured) (bool, error) {
	v, found, err := unstructured.NestedString(obj.Object, strings.Split(m.Field, ".")...)
	if err != nil {
		return false, err
	}

	return found && v == m.Value, nil
}

func (f *FieldSelector) Matches(f2 fields.Fields) bool {
	for _, matchField := range f.MatchFields {
		if !matchField.Matches(f2) {
			return false
		}
	}

	return true
}

func (f *FieldSelector) Empty() bool {
	return len(f.MatchFields) == 0
}

func (f *FieldSelector) RequiresExactMatch(field string) (value string, found bool) {
	for i := range f.MatchFields {
		if value, found = f.MatchFields[i].RequiresExactMatch(field); found {
			return
		}
	}

	return
}

func (f *FieldSelector) Transform(fn fields.TransformFunc) (fields.Selector, error) {
	next := make([]*MatchField, 0, len(f.MatchFields))
	for _, s := range f.MatchFields {
		n, err := s.Transform(fn)
		if err != nil {
			return nil, err
		}
		if !n.Empty() {
			next = append(next, n.(*MatchField))
		}
	}
	return &FieldSelector{next}, nil
}

func (f *FieldSelector) Requirements() fields.Requirements {
	reqs := make([]fields.Requirement, 0, len(f.MatchFields))
	for _, s := range f.MatchFields {
		rs := s.Requirements()
		reqs = append(reqs, rs...)
	}
	return reqs
}

func (f *FieldSelector) String() string {
	var terms []string
	for _, q := range f.MatchFields {
		terms = append(terms, q.String())
	}
	return strings.Join(terms, ",")
}

func (f *FieldSelector) DeepCopySelector() fields.Selector {
	if f == nil {
		return nil
	}
	out := make([]*MatchField, len(f.MatchFields))
	for i := range f.MatchFields {
		out[i] = f.MatchFields[i].DeepCopySelector().(*MatchField)
	}
	return &FieldSelector{out}
}

func (f *FieldSelector) MatchObject(obj *unstructured.Unstructured) (bool, error) {
	for i := range f.MatchFields {
		match, err := f.MatchFields[i].MatchObject(obj)
		if err != nil {
			return false, err
		}

		if !match {
			return match, nil
		}
	}

	return true, nil
}
