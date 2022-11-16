package model

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
)

func Test_handlePath(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "normal",
			args: args{path: "/spec/template/spec/containers/0/tolerations/1/key-1"},
			want: "spec.template.spec.containers[0].tolerations[1].\"key-1\"",
		},
		{
			name: "no-hande",
			args: args{path: "a.b.c.d[0]"},
			want: "a.b.c.d[0]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := handlePath(tt.args.path); got != tt.want {
				t.Errorf("handlePath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOverrideRulesToOverridePolicyRenderData(t *testing.T) {
	strPrt := func(s string) *string {
		return &s
	}
	type args struct {
		or *policyv1alpha1.OverrideRuleTemplate
	}
	tests := []struct {
		name string
		args args
		want *OverridePolicyRenderData
	}{
		{
			name: "ann1",
			args: args{
				or: &policyv1alpha1.OverrideRuleTemplate{
					Type:      policyv1alpha1.OverrideRuleTypeAnnotations,
					Operation: policyv1alpha1.OverriderOpReplace,
					Path:      "anno-1",
					Value:     &policyv1alpha1.ConstantValue{String: strPrt("cue")},
				},
			},
			want: &OverridePolicyRenderData{
				Type:      policyv1alpha1.OverrideRuleTypeAnnotations,
				Op:        policyv1alpha1.OverriderOpReplace,
				Path:      "anno-1",
				Value:     "cue",
				ValueType: policyv1alpha1.ValueTypeConst,
			},
		},
		{
			name: "label1",
			args: args{
				or: &policyv1alpha1.OverrideRuleTemplate{
					Type:      policyv1alpha1.OverrideRuleTypeLabels,
					Operation: policyv1alpha1.OverriderOpReplace,
					Path:      "anno-1",
					Value:     &policyv1alpha1.ConstantValue{String: strPrt("cue")},
				},
			},
			want: &OverridePolicyRenderData{
				Type:      policyv1alpha1.OverrideRuleTypeLabels,
				Op:        policyv1alpha1.OverriderOpReplace,
				Path:      "anno-1",
				Value:     "cue",
				ValueType: policyv1alpha1.ValueTypeConst,
			},
		},

		{
			name: "oversell",
			args: args{
				or: &policyv1alpha1.OverrideRuleTemplate{
					Type:      policyv1alpha1.OverrideRuleTypeResourcesOversell,
					Operation: policyv1alpha1.OverriderOpReplace,
					ResourcesOversell: &policyv1alpha1.ResourcesOversellRule{
						CpuFactor:    "0.5",
						MemoryFactor: "0.2",
						DiskFactor:   "0.1",
					},
				},
			},
			want: &OverridePolicyRenderData{
				Type: policyv1alpha1.OverrideRuleTypeResourcesOversell,
				Op:   policyv1alpha1.OverriderOpReplace,
				ResourcesOversell: &policyv1alpha1.ResourcesOversellRule{
					CpuFactor:    "0.5",
					MemoryFactor: "0.2",
					DiskFactor:   "0.1",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := OverrideRulesToOverridePolicyRenderData(tt.args.or); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("OverrideRulesToOverridePolicyRenderData() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOverridePolicyRenderData_String(t *testing.T) {
	type fields struct {
		Type              policyv1alpha1.OverrideRuleType
		Op                policyv1alpha1.OverriderOperator
		Path              string
		Value             any
		ValueType         policyv1alpha1.ValueType
		ValueRef          *ResourceRefer
		Resources         *corev1.ResourceRequirements
		ResourcesOversell *policyv1alpha1.ResourcesOversellRule
		Tolerations       []*corev1.Toleration
		Affinity          *corev1.Affinity
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "1",
			fields: fields{
				Type: policyv1alpha1.OverrideRuleTypeAnnotations,
			},
			want: `{
	"Type": "annotations",
	"Op": "",
	"Path": "",
	"Value": null,
	"ValueType": "",
	"ValueRef": null,
	"Resources": null,
	"ResourcesOversell": null,
	"Tolerations": null,
	"Affinity": null
}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mrd := &OverridePolicyRenderData{
				Type:              tt.fields.Type,
				Op:                tt.fields.Op,
				Path:              tt.fields.Path,
				Value:             tt.fields.Value,
				ValueType:         tt.fields.ValueType,
				ValueRef:          tt.fields.ValueRef,
				Resources:         tt.fields.Resources,
				ResourcesOversell: tt.fields.ResourcesOversell,
				Tolerations:       tt.fields.Tolerations,
				Affinity:          tt.fields.Affinity,
			}
			if got := mrd.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}
