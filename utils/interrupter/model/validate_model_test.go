package model

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/util/intstr"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
)

func TestValidatePolicyRenderData_String(t *testing.T) {

	type fields struct {
		Type      string
		Condition *ValidateCondition
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "string",
			fields: fields{
				Type: policyv1alpha1.ValidateRuleTypeCondition,
				Condition: &ValidateCondition{
					Cond: string(policyv1alpha1.CondExist),
					DataRef: &ResourceRefer{
						From:         policyv1alpha1.FromCurrentObject,
						CueObjectKey: "object",
						Path:         "spec.replica",
					},
					Message: "no pass",
				},
			},
			want: `{
	"Type": "condition",
	"Condition": {
		"Cond": "Exist",
		"Value": null,
		"ValueType": "",
		"ValueRef": null,
		"DataRef": {
			"From": "current",
			"CueObjectKey": "object",
			"Path": "spec.replica"
		},
		"ValueProcess": null,
		"Message": "no pass"
	}
}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vrd := &ValidatePolicyRenderData{
				Type:      tt.fields.Type,
				Condition: tt.fields.Condition,
			}
			if got := vrd.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_convertCond(t *testing.T) {
	type args struct {
		c policyv1alpha1.Cond
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "exit",
			args: args{
				c: policyv1alpha1.CondExist,
			},
			want: "Exist",
		},
		{
			name: "gte",
			args: args{
				c: policyv1alpha1.CondGreaterOrEqual,
			},
			want: ">=",
		},
		{
			name: "gt",
			args: args{
				c: policyv1alpha1.CondGreater,
			},
			want: ">",
		},
		{
			name: "lte",
			args: args{
				c: policyv1alpha1.CondLesserOrEqual,
			},
			want: "<=",
		},
		{
			name: "lt",
			args: args{
				c: policyv1alpha1.CondLesser,
			},
			want: "<",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := convertCond(tt.args.c); got != tt.want {
				t.Errorf("convertCond() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_convertGeneralCondition(t *testing.T) {
	ip := func(i int64) *int64 {
		return &i
	}
	type args struct {
		vc *policyv1alpha1.ValidateCondition
	}
	tests := []struct {
		name string
		args args
		want *ValidateCondition
	}{
		{
			name: "1",
			args: args{
				vc: &policyv1alpha1.ValidateCondition{
					AffectMode: policyv1alpha1.AffectModeAllow,
					Cond:       policyv1alpha1.CondGreaterOrEqual,
					DataRef: &policyv1alpha1.ResourceRefer{
						From: policyv1alpha1.FromCurrentObject,
						Path: "/spec/replica",
					},
					Message: "no",
					Value:   &policyv1alpha1.ConstantValue{Integer: ip(10)},
				},
			},
			want: &ValidateCondition{
				Cond:      ">=",
				Value:     &policyv1alpha1.ConstantValue{Integer: ip(10)},
				ValueType: policyv1alpha1.ValueTypeConst,
				DataRef: &ResourceRefer{
					From:         policyv1alpha1.FromCurrentObject,
					CueObjectKey: "object",
					Path:         "spec.replica",
				},
				Message: "no",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := convertGeneralCondition(tt.args.vc); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertGeneralCondition() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_convertResourceRefer(t *testing.T) {
	type args struct {
		suffix string
		rf     *policyv1alpha1.ResourceRefer
	}
	tests := []struct {
		name string
		args args
		want *ResourceRefer
	}{
		{
			name: "1",
			args: args{
				rf: &policyv1alpha1.ResourceRefer{
					From: policyv1alpha1.FromCurrentObject,
					Path: "/spec/replica",
				},
			},
			want: &ResourceRefer{
				From:         policyv1alpha1.FromCurrentObject,
				CueObjectKey: "object",
				Path:         "spec.replica",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := convertResourceRefer(tt.args.suffix, tt.args.rf); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertResourceRefer() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_intStr2FloatPtr(t *testing.T) {
	fPtr := func(f float64) *float64 {
		return &f
	}
	type args struct {
		is *intstr.IntOrString
	}
	tests := []struct {
		name  string
		args  args
		want  *float64
		want1 bool
	}{
		{
			name: "int",
			args: args{
				is: &intstr.IntOrString{
					Type:   intstr.Int,
					IntVal: 10,
				},
			},
			want:  fPtr(10),
			want1: false,
		},
		{
			name: "float",
			args: args{
				is: &intstr.IntOrString{
					Type:   intstr.String,
					StrVal: "1.2",
				},
			},
			want:  fPtr(1.2),
			want1: false,
		},
		{
			name: "percent",
			args: args{
				is: &intstr.IntOrString{
					Type:   intstr.String,
					StrVal: "30%",
				},
			},
			want:  fPtr(0.3),
			want1: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := intStr2FloatPtr(tt.args.is)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("intStr2FloatPtr() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("intStr2FloatPtr() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
