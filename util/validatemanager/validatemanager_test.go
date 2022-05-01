package validatemanager

import (
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/test/helper"
	"github.com/k-cloud-labs/pkg/test/mock"
	"github.com/k-cloud-labs/pkg/util"
	utilhelper "github.com/k-cloud-labs/pkg/util/converter"
)

func TestValidateManagerImpl_ApplyValidatePolicies(t *testing.T) {
	pod := helper.NewPod(metav1.NamespaceDefault, "test")
	podObj, _ := utilhelper.ToUnstructured(pod)

	validatePolicy1 := &policyv1alpha1.ClusterValidatePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "validatepolicy1",
		},
		Spec: policyv1alpha1.ClusterValidatePolicySpec{
			ValidateRules: []policyv1alpha1.ValidateRuleWithOperation{
				{
					TargetOperations: []string{util.Delete},
					Cue: `
object: _ @tag(object)

validate: {
	valid: object.metadata.name == "ut-validate-policy-success"
}
`,
				}}}}

	tests := []struct {
		name         string
		operation    string
		resource     *unstructured.Unstructured
		policy       *policyv1alpha1.ClusterValidatePolicy
		wantedResult *ValidateResult
		wantedErr    error
	}{
		{
			name:      "ut-validate-policy-success",
			policy:    validatePolicy1,
			operation: util.Delete,
			resource:  podObj,
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
	}, nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := m.ApplyValidatePolicies(tt.resource, tt.operation)
			if !reflect.DeepEqual(result, tt.wantedResult) || !reflect.DeepEqual(err, tt.wantedErr) {
				t.Errorf("ApplyValidatePolicies() = %v, %v want %v, %v", result, err, tt.wantedResult, tt.wantedErr)
			}
		})
	}
}
