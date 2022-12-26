package utils

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
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
					"a1": "v1",
					"a2": "v2",
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
		selectors []v1alpha1.ResourceSelector
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
				selectors: []v1alpha1.ResourceSelector{
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
				selectors: []v1alpha1.ResourceSelector{
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
				selectors: []v1alpha1.ResourceSelector{
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
				selectors: []v1alpha1.ResourceSelector{
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
				selectors: []v1alpha1.ResourceSelector{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						FieldSelector: &v1alpha1.FieldSelector{
							MatchFields: []*v1alpha1.MatchField{
								{
									Field: "spec.field1",
									Value: "v1",
								},
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
				selectors: []v1alpha1.ResourceSelector{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						FieldSelector: &v1alpha1.FieldSelector{
							MatchFields: []*v1alpha1.MatchField{
								{
									Field: "spec.field1",
									Value: "v1",
								},
								{
									Field: "spec.field2",
									Value: "v2",
								},
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
				selectors: []v1alpha1.ResourceSelector{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						FieldSelector: &v1alpha1.FieldSelector{
							MatchFields: []*v1alpha1.MatchField{
								{
									Field: "metadata.annotations.a1",
									Value: "v1",
								},
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "labelsAndField1",
			args: args{
				resource: newObjectForSelector(),
				selectors: []v1alpha1.ResourceSelector{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"l1": "v1",
								"l2": "v2",
							},
						},
						FieldSelector: &v1alpha1.FieldSelector{
							MatchFields: []*v1alpha1.MatchField{
								{
									Field: "status.field1",
									Value: "v1",
								},
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
				selectors: []v1alpha1.ResourceSelector{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"l1": "v1",
								"l2": "v2",
							},
						},
						FieldSelector: &v1alpha1.FieldSelector{
							MatchFields: []*v1alpha1.MatchField{
								{
									Field: "metadata.annotations.field1",
									Value: "v1",
								},
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
				selectors: []v1alpha1.ResourceSelector{
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
