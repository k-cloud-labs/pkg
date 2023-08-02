package utils

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
)

func newObjectForSelector() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "pod1",
				"namespace": "default",
				"labels": map[string]interface{}{
					"l1": "v1",
					"l2": "v2",
				},
				"annotations": map[string]interface{}{
					"a1.b1/c1": "v1",
					"a2.b2/c2": "v2",
				},
			},
			"spec": map[string]interface{}{
				"field1": "v1",
			},
			"status": map[string]interface{}{
				"field1": "v1",
			},
		},
	}
}

func TestResourceMatchSelectors(t *testing.T) {
	type args struct {
		resource  *unstructured.Unstructured
		selectors []policyv1alpha1.ResourceSelector
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "name",
			args: args{
				resource: newObjectForSelector(),
				selectors: []policyv1alpha1.ResourceSelector{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						Namespace:  "default",
						Name:       "pod1",
					},
				},
			},
			want: true,
		},
		{
			name: "labels1",
			args: args{
				resource: newObjectForSelector(),
				selectors: []policyv1alpha1.ResourceSelector{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"l1": "v1",
								"l2": "v2",
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "labels2",
			args: args{
				resource: newObjectForSelector(),
				selectors: []policyv1alpha1.ResourceSelector{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						LabelSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      "l1",
									Operator: metav1.LabelSelectorOpExists,
								},
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "labels3",
			args: args{
				resource: newObjectForSelector(),
				selectors: []policyv1alpha1.ResourceSelector{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"l1": "v1",
								"l2": "v2",
								"l3": "v3",
							},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "field1",
			args: args{
				resource: newObjectForSelector(),
				selectors: []policyv1alpha1.ResourceSelector{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						FieldSelector: &policyv1alpha1.FieldSelector{
							MatchFields: map[string]string{
								"spec.field1": "v1",
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "field2",
			args: args{
				resource: newObjectForSelector(),
				selectors: []policyv1alpha1.ResourceSelector{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						FieldSelector: &policyv1alpha1.FieldSelector{
							MatchFields: map[string]string{
								"spec.field1": "v1", // valid
								"spec.field2": "v2", // invalid
							},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "fieldAnnotation",
			args: args{
				resource: newObjectForSelector(),
				selectors: []policyv1alpha1.ResourceSelector{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						FieldSelector: &policyv1alpha1.FieldSelector{
							MatchFields: map[string]string{
								"metadata.annotations.a1.b1/c1": "v1",
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "fieldExpressionExist",
			args: args{
				resource: newObjectForSelector(),
				selectors: []policyv1alpha1.ResourceSelector{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						FieldSelector: &policyv1alpha1.FieldSelector{
							MatchExpressions: []*policyv1alpha1.FieldSelectorRequirement{
								{
									Field:    "metadata.annotations.a1.b1/c1",
									Operator: metav1.LabelSelectorOpExists,
								},
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "fieldExpressionNotExist",
			args: args{
				resource: newObjectForSelector(),
				selectors: []policyv1alpha1.ResourceSelector{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						FieldSelector: &policyv1alpha1.FieldSelector{
							MatchExpressions: []*policyv1alpha1.FieldSelectorRequirement{
								{
									Field:    "metadata.annotations.aaa",
									Operator: metav1.LabelSelectorOpDoesNotExist,
								},
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "fieldExpressionIn",
			args: args{
				resource: newObjectForSelector(),
				selectors: []policyv1alpha1.ResourceSelector{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						FieldSelector: &policyv1alpha1.FieldSelector{
							MatchExpressions: []*policyv1alpha1.FieldSelectorRequirement{
								{
									Field:    "metadata.annotations.a1.b1/c1",
									Operator: metav1.LabelSelectorOpIn,
									Value:    []string{"v1", "v2", "v3"},
								},
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "fieldExpressionNotIn",
			args: args{
				resource: newObjectForSelector(),
				selectors: []policyv1alpha1.ResourceSelector{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						FieldSelector: &policyv1alpha1.FieldSelector{
							MatchExpressions: []*policyv1alpha1.FieldSelectorRequirement{
								{
									Field:    "metadata.annotations.a1.b1/c1",
									Operator: metav1.LabelSelectorOpNotIn,
									Value:    []string{"v4", "v5", "v6"},
								},
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "fieldExpressionInvalid",
			args: args{
				resource: newObjectForSelector(),
				selectors: []policyv1alpha1.ResourceSelector{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						FieldSelector: &policyv1alpha1.FieldSelector{
							MatchExpressions: []*policyv1alpha1.FieldSelectorRequirement{
								{
									Field:    "metadata.annotations.a1.b1/c1",
									Operator: "!=", // not support
									Value:    []string{"v4"},
								},
							},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "labelsAndField1",
			args: args{
				resource: newObjectForSelector(),
				selectors: []policyv1alpha1.ResourceSelector{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"l1": "v1",
								"l2": "v2",
							},
						},
						FieldSelector: &policyv1alpha1.FieldSelector{
							MatchFields: map[string]string{
								"status.field1": "v1",
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "labelsAndField2",
			args: args{
				resource: newObjectForSelector(),
				selectors: []policyv1alpha1.ResourceSelector{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"l1": "v1",
								"l2": "v2",
							}, // valid
						},
						FieldSelector: &policyv1alpha1.FieldSelector{
							MatchFields: map[string]string{
								"metadata.annotations.field1": "v1", // invalid
							},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "all",
			args: args{
				resource: newObjectForSelector(),
				selectors: []policyv1alpha1.ResourceSelector{
					{
						APIVersion: "v1",
						Kind:       "Pod",
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResourceMatchSelectors(tt.args.resource, tt.args.selectors...); got != tt.want {
				t.Errorf("ResourceMatchSelectors() = %v, want %v", got, tt.want)
			}
		})
	}
}
