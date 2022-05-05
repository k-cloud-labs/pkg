package validatemanager

import (
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/test/helper"
	"github.com/k-cloud-labs/pkg/test/mock"
	utilhelper "github.com/k-cloud-labs/pkg/util/converter"
)

func TestValidateManagerImpl_ApplyValidatePolicies(t *testing.T) {
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
	valid: object.metadata.name == "ut-validate-policy-success"
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
				Valid: true,
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
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cvpLister := mock.NewMockClusterValidatePolicyLister(ctrl)
	m := NewValidateManager(cvpLister)

	cvpLister.EXPECT().List(labels.Everything()).Return([]*policyv1alpha1.ClusterValidatePolicy{
		validatePolicy1,
		validatePolicy2,
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
