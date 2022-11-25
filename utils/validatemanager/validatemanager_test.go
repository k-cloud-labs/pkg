package validatemanager

import (
	"flag"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/test/helper"
	"github.com/k-cloud-labs/pkg/test/mock"
	"github.com/k-cloud-labs/pkg/utils"
	"github.com/k-cloud-labs/pkg/utils/cue"
	utilhelper "github.com/k-cloud-labs/pkg/utils/util"
)

func TestValidateManagerImpl_ApplyValidatePolicies(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ExitOnError)
	klog.InitFlags(fs)
	if err := fs.Parse([]string{"-v", "4"}); err != nil {
		t.Errorf("parse flag err=%v", err)
		return
	}
	pod := helper.NewPod(metav1.NamespaceDefault, "test")
	podObj, _ := utilhelper.ToUnstructured(pod)
	oldPod := pod.DeepCopy()
	oldPodObj, _ := utilhelper.ToUnstructured(oldPod)

	validatePolicy1 := &policyv1alpha1.ClusterValidatePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "validatepolicy1",
		},
		Spec: policyv1alpha1.ClusterValidatePolicySpec{
			ValidateRules: []policyv1alpha1.ValidateRuleWithOperation{
				{
					TargetOperations: []admissionv1.Operation{admissionv1.Delete},
					Cue: `
object: _ @tag(object)

validate: {
	valid: object.metadata.name != "ut-validate-policy-success"
}
`,
				}}}}
	validatePolicy2 := &policyv1alpha1.ClusterValidatePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "validatepolicy2",
		},
		Spec: policyv1alpha1.ClusterValidatePolicySpec{
			ValidateRules: []policyv1alpha1.ValidateRuleWithOperation{
				{
					TargetOperations: []admissionv1.Operation{admissionv1.Update},
					Cue: `
object: _ @tag(object)
oldObject: _ @tag(oldObject)

validate: {
	valid: oldObject.metadata.annotations == _|_ && object.metadata.annotations == _|_
}
`,
				}}}}

	validatePolicy3 := &policyv1alpha1.ClusterValidatePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "validatepolicy3",
		},
		Spec: policyv1alpha1.ClusterValidatePolicySpec{
			ValidateRules: []policyv1alpha1.ValidateRuleWithOperation{
				{
					TargetOperations: []admissionv1.Operation{admissionv1.Delete},
					Template: &policyv1alpha1.ValidateRuleTemplate{
						Type: policyv1alpha1.ValidateRuleTypeCondition,
						Condition: &policyv1alpha1.ValidateCondition{
							Cond:    policyv1alpha1.CondExist,
							Message: "cannot delete this ns",
						},
					},
					RenderedCue: `
data:      _ @tag(data)
object:    data.object
oldObject: data.oldObject
validate: {
    if object.metadata.annotations == _|_ {
        valid:   false
    }
}
`,
				}}}}

	validatePolicy4 := &policyv1alpha1.ClusterValidatePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "validatepolicy4",
		},
		Spec: policyv1alpha1.ClusterValidatePolicySpec{
			ValidateRules: []policyv1alpha1.ValidateRuleWithOperation{
				{
					TargetOperations: []admissionv1.Operation{admissionv1.Delete},
					Template: &policyv1alpha1.ValidateRuleTemplate{
						Type: policyv1alpha1.ValidateRuleTypePodAvailableBadge,
						PodAvailableBadge: &policyv1alpha1.PodAvailableBadge{
							MaxUnavailable: &intstr.IntOrString{
								Type:   intstr.String,
								StrVal: "40%",
							},
						},
					},
					RenderedCue: ``,
				}}}}

	tests := []struct {
		name         string
		operation    admissionv1.Operation
		object       *unstructured.Unstructured
		oldObject    *unstructured.Unstructured
		wantedResult *ValidateResult
		wantedErr    error
	}{
		{
			name:      "ut-validate-policy-success-delete",
			operation: admissionv1.Delete,
			object:    podObj,
			oldObject: nil,
			wantedErr: nil,
			wantedResult: &ValidateResult{
				Valid: false,
			},
		},
		{
			name:      "ut-validate-policy-success-update",
			operation: admissionv1.Update,
			object:    podObj,
			oldObject: oldPodObj,
			wantedErr: nil,
			wantedResult: &ValidateResult{
				Valid: true,
			},
		},
		{
			name:      "ut-validate-policy-success-delete-rendercue",
			operation: admissionv1.Delete,
			object:    podObj,
			oldObject: nil,
			wantedErr: nil,
			wantedResult: &ValidateResult{
				Valid: false,
			},
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cvpLister := mock.NewMockClusterValidatePolicyLister(ctrl)
	m := NewValidateManager(nil, cvpLister)

	cvpLister.EXPECT().List(labels.Everything()).Return([]*policyv1alpha1.ClusterValidatePolicy{
		validatePolicy1,
		validatePolicy2,
		validatePolicy3,
		validatePolicy4,
	}, nil).AnyTimes()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := m.ApplyValidatePolicies(tt.object, tt.oldObject, tt.operation)
			if !reflect.DeepEqual(result, tt.wantedResult) || !reflect.DeepEqual(err, tt.wantedErr) {
				t.Errorf("ApplyValidatePolicies() = %v, %v want %v, %v", result, err, tt.wantedResult, tt.wantedErr)
			}
		})
	}
}

func Test_executeCueV2(t *testing.T) {
	type args struct {
		cueStr     string
		parameters []cue.Parameter
	}
	tests := []struct {
		name    string
		args    args
		want    *ValidateResult
		wantErr bool
	}{
		{
			name: "normal",
			args: args{
				cueStr: `
data: _ @tag(data)
validate: {
if data.object.name == "cue" {
		valid: false
		reason: "name cannot be cue"
}
}
				`,
				parameters: []cue.Parameter{
					{
						Name: utils.DataParameterName,
						Object: map[string]any{
							"object": map[string]any{
								"name": "cue",
							},
						},
					},
				},
			},
			want: &ValidateResult{
				Reason: "name cannot be cue",
				Valid:  false,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := executeCueV2(tt.args.cueStr, tt.args.parameters)
			if (err != nil) != tt.wantErr {
				t.Errorf("executeCueV2() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("executeCueV2() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getPodPhase(t *testing.T) {
	type args struct {
		obj *unstructured.Unstructured
	}
	tests := []struct {
		name string
		args args
		want corev1.PodPhase
	}{
		{
			name: "1",
			args: args{
				obj: nil,
			},
			want: "",
		},
		{
			name: "2",
			args: args{
				obj: &unstructured.Unstructured{
					Object: map[string]any{
						"kind": "Deployment",
					},
				},
			},
			want: "",
		},
		{
			name: "3",
			args: args{
				obj: &unstructured.Unstructured{
					Object: map[string]any{
						"kind": "Pod",
					},
				},
			},
			want: "",
		},
		{
			name: "4",
			args: args{
				obj: &unstructured.Unstructured{
					Object: map[string]any{
						"kind": "Pod",
						"status": map[string]any{
							"abc": "abc",
						},
					},
				},
			},
			want: "",
		},
		{
			name: "5",
			args: args{
				obj: &unstructured.Unstructured{
					Object: map[string]any{
						"kind": "Pod",
						"status": map[string]any{
							"phase": "Pending",
						},
					},
				},
			},
			want: corev1.PodPending,
		},
		{
			name: "6",
			args: args{
				obj: &unstructured.Unstructured{
					Object: map[string]any{
						"kind": "Pod",
						"status": map[string]any{
							"phase": "Running",
						},
					},
				},
			},
			want: corev1.PodRunning,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getPodPhase(tt.args.obj); got != tt.want {
				t.Errorf("getPodPhase() = %v, want %v", got, tt.want)
			}
		})
	}
}
