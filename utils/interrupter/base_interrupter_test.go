package interrupter

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/util/intstr"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/utils/templatemanager"
	"github.com/k-cloud-labs/pkg/utils/templatemanager/templates"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func test_baseInterrupter() (*baseInterrupter, error) {
	mtm, err := templatemanager.NewOverrideTemplateManager(&templatemanager.TemplateSource{
		Content:      templates.OverrideTemplate,
		TemplateName: "BaseTemplate",
	})
	if err != nil {
		return nil, err
	}

	vtm, err := templatemanager.NewValidateTemplateManager(&templatemanager.TemplateSource{
		Content:      templates.ValidateTemplate,
		TemplateName: "BaseTemplate",
	})
	if err != nil {
		return nil, err
	}
	return NewBaseInterrupter(mtm, vtm, templatemanager.NewCueManager()).(*baseInterrupter), nil
}

func Test_baseInterrupter_renderAndFormat(t *testing.T) {
	bi, err := test_baseInterrupter()
	if err != nil {
		t.Error(err)
		return
	}

	intPtr := func(i int64) *int64 {
		return &i
	}

	type args struct {
		data any
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "validatePolicy",
			args: args{
				data: &policyv1alpha1.ValidateRuleTemplate{
					Type: policyv1alpha1.ValidateRuleTypeCondition,
					Condition: &policyv1alpha1.ValidateCondition{
						AffectMode: policyv1alpha1.AffectModeReject,
						Cond:       policyv1alpha1.CondLesser,
						DataRef: &policyv1alpha1.ResourceRefer{
							From: policyv1alpha1.FromCurrentObject,
							Path: "/spec/replica",
						},
						Message: "no deletion",
						Value: &policyv1alpha1.ConstantValue{
							Integer: intPtr(100),
						},
						ValueProcess: &policyv1alpha1.ValueProcess{
							Operation: policyv1alpha1.OperationTypeMultiply,
							OperationWith: &intstr.IntOrString{
								Type:   intstr.String,
								StrVal: "60%",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "overridePolicy",
			args: args{
				data: &policyv1alpha1.OverrideRuleTemplate{
					Type:      policyv1alpha1.OverrideRuleTypeAnnotations,
					Operation: policyv1alpha1.OverriderOpReplace,
					Value: &policyv1alpha1.ConstantValue{
						StringMap: map[string]string{
							"app":       "cue",
							"no-delete": "True",
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotB, err := bi.renderAndFormat(tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("renderAndFormat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			t.Logf("cue >>>\n%v", string(gotB))
		})
	}
}

func Test_convertToPolicy(t *testing.T) {
	type args struct {
		u    *unstructured.Unstructured
		data any
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "1",
			args: args{
				u: &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "OverridePolicy",
				}},
				data: &policyv1alpha1.OverridePolicy{},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := convertToPolicy(tt.args.u, tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("convertToPolicy() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_trimBlankLine(t *testing.T) {
	type args struct {
		data []byte
	}
	tests := []struct {
		name string
		args args
		want []byte
	}{
		{
			name: "0",
			args: args{
				data: []byte(`abc

abc`),
			},
			want: []byte(`abc
abc`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := trimBlankLine(tt.args.data); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("trimBlankLine() = %v, want %v", string(got), string(tt.want))
			}
		})
	}
}
