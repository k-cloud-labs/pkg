package overridemanager

import (
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
	v1alpha10 "github.com/k-cloud-labs/pkg/client/listers/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/test/helper"
	"github.com/k-cloud-labs/pkg/test/mock"
	"github.com/k-cloud-labs/pkg/util"
	utilhelper "github.com/k-cloud-labs/pkg/util/converter"
)

func TestGetMatchingOverridePolicies(t *testing.T) {
	deployment := helper.NewDeployment(metav1.NamespaceDefault, "test")
	deploymentObj, _ := utilhelper.ToUnstructured(deployment)

	overriders1 := policyv1alpha1.Overriders{
		Plaintext: []policyv1alpha1.PlaintextOverrider{
			{
				Path:     "/metadata/annotations",
				Operator: "add",
				Value:    apiextensionsv1.JSON{Raw: []byte("foo: bar")},
			},
		},
	}
	overriders2 := policyv1alpha1.Overriders{
		Plaintext: []policyv1alpha1.PlaintextOverrider{
			{
				Path:     "/metadata/annotations",
				Operator: "add",
				Value:    apiextensionsv1.JSON{Raw: []byte("aaa: bbb")},
			},
		},
	}
	overriders3 := policyv1alpha1.Overriders{
		Plaintext: []policyv1alpha1.PlaintextOverrider{
			{
				Path:     "/metadata/annotations",
				Operator: "add",
				Value:    apiextensionsv1.JSON{Raw: []byte("hello: world")},
			},
		},
	}
	overridePolicy1 := &policyv1alpha1.OverridePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceDefault,
			Name:      "overridePolicy1",
		},
		Spec: policyv1alpha1.OverridePolicySpec{
			ResourceSelectors: []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			},
			OverrideRules: []policyv1alpha1.RuleWithOperation{
				{
					TargetOperations: []string{util.Create},
					Overriders:       overriders1,
				},
				{
					TargetOperations: []string{util.Update},
					Overriders:       overriders2,
				},
			},
		},
	}
	overridePolicy2 := &policyv1alpha1.OverridePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceDefault,
			Name:      "overridePolicy2",
		},
		Spec: policyv1alpha1.OverridePolicySpec{
			OverrideRules: []policyv1alpha1.RuleWithOperation{
				{
					TargetOperations: []string{util.Create, util.Update},
					Overriders:       overriders3,
				},
			},
		},
	}
	overridePolicy3 := &policyv1alpha1.OverridePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceDefault,
			Name:      "overridePolicy3",
		},
		Spec: policyv1alpha1.OverridePolicySpec{
			OverrideRules: []policyv1alpha1.RuleWithOperation{
				{
					TargetOperations: nil,
					Overriders:       overriders3,
				},
			},
		},
	}

	m := &overrideManagerImpl{}
	tests := []struct {
		name             string
		policies         []GeneralOverridePolicy
		resource         *unstructured.Unstructured
		operation        string
		wantedOverriders []policyOverriders
	}{
		{
			name:      "OverrideRules test 1",
			policies:  []GeneralOverridePolicy{overridePolicy1, overridePolicy2, overridePolicy3},
			resource:  deploymentObj,
			operation: util.Create,
			wantedOverriders: []policyOverriders{
				{
					name:       overridePolicy1.Name,
					namespace:  overridePolicy1.Namespace,
					overriders: overriders1,
				},
				{
					name:       overridePolicy2.Name,
					namespace:  overridePolicy2.Namespace,
					overriders: overriders3,
				},
				{
					name:       overridePolicy3.Name,
					namespace:  overridePolicy3.Namespace,
					overriders: overriders3,
				},
			},
		},
		{
			name:      "OverrideRules test 2",
			policies:  []GeneralOverridePolicy{overridePolicy1, overridePolicy2, overridePolicy3},
			resource:  deploymentObj,
			operation: util.Update,
			wantedOverriders: []policyOverriders{
				{
					name:       overridePolicy1.Name,
					namespace:  overridePolicy1.Namespace,
					overriders: overriders2,
				},
				{
					name:       overridePolicy2.Name,
					namespace:  overridePolicy2.Namespace,
					overriders: overriders3,
				},
				{
					name:       overridePolicy3.Name,
					namespace:  overridePolicy3.Namespace,
					overriders: overriders3,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := m.getOverridersFromOverridePolicies(tt.policies, tt.resource, tt.operation); !reflect.DeepEqual(got, tt.wantedOverriders) {
				t.Errorf("getOverridersFromOverridePolicies() = %v, want %v", got, tt.wantedOverriders)
			}
		})
	}
}

func TestOverrideManagerImpl_ApplyOverridePolicies(t *testing.T) {
	deployment := helper.NewDeployment(metav1.NamespaceDefault, "test")
	deploymentObj, _ := utilhelper.ToUnstructured(deployment)

	overriders1 := policyv1alpha1.Overriders{
		Plaintext: []policyv1alpha1.PlaintextOverrider{
			{
				Path:     "/metadata/annotations",
				Operator: "add",
				Value:    apiextensionsv1.JSON{Raw: []byte("{\"foo\": \"bar\"}")},
			},
		},
	}
	overriders2 := policyv1alpha1.Overriders{
		Plaintext: []policyv1alpha1.PlaintextOverrider{
			{
				Path:     "/metadata/annotations/aaa",
				Operator: "add",
				//Value:    apiextensionsv1.JSON{Raw: []byte("{\"aaa\": \"bbb\"}")},
				Value: apiextensionsv1.JSON{Raw: []byte("\"bbb\"")},
			},
		},
		Cue: `
object: _ @tag(object)

patches: [{
	path: "/metadata/annotations/cue",
	op: "add",
	value: "cue",
}]
`,
	}
	overriders3 := policyv1alpha1.Overriders{
		Plaintext: []policyv1alpha1.PlaintextOverrider{
			{
				Path:     "/metadata/annotations",
				Operator: "add",
				Value:    apiextensionsv1.JSON{Raw: []byte("{\"hello\": \"world\"}")},
			},
		},
	}

	overridePolicy1 := &policyv1alpha1.OverridePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceDefault,
			Name:      "overridePolicy1",
		},
		Spec: policyv1alpha1.OverridePolicySpec{
			ResourceSelectors: []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			},
			OverrideRules: []policyv1alpha1.RuleWithOperation{
				{
					TargetOperations: []string{util.Create},
					Overriders:       overriders1,
				},
			},
		},
	}
	overridePolicy2 := &policyv1alpha1.OverridePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceDefault,
			Name:      "overridePolicy2",
		},
		Spec: policyv1alpha1.OverridePolicySpec{
			ResourceSelectors: []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			},
			OverrideRules: []policyv1alpha1.RuleWithOperation{
				{
					TargetOperations: []string{util.Create},
					Overriders:       overriders2,
				},
			},
		},
	}
	overridePolicy3 := &policyv1alpha1.ClusterOverridePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "overridePolicy3",
		},
		Spec: policyv1alpha1.OverridePolicySpec{
			OverrideRules: []policyv1alpha1.RuleWithOperation{
				{
					TargetOperations: []string{util.Create, util.Update},
					Overriders:       overriders3,
				},
			},
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	opLister := mock.NewMockOverridePolicyLister(ctrl)
	copLister := mock.NewMockClusterOverridePolicyLister(ctrl)
	m := NewOverrideManager(copLister, opLister)

	opLister.EXPECT().List(labels.Everything()).Return([]*policyv1alpha1.OverridePolicy{
		overridePolicy1,
		overridePolicy2,
	}, nil)

	copLister.EXPECT().List(labels.Everything()).Return([]*policyv1alpha1.ClusterOverridePolicy{
		overridePolicy3,
	}, nil)

	tests := []struct {
		name              string
		opLister          v1alpha10.OverridePolicyLister
		copLister         v1alpha10.ClusterOverridePolicyLister
		resource          *unstructured.Unstructured
		operation         string
		wantedCOPs        *AppliedOverrides
		wantedOPs         *AppliedOverrides
		wantedAnnotations map[string]string
		wantedErr         error
	}{
		{
			name:      "OverrideRules test 1",
			opLister:  opLister,
			copLister: copLister,
			resource:  deploymentObj,
			operation: util.Create,
			wantedErr: nil,
			wantedOPs: &AppliedOverrides{
				AppliedItems: []OverridePolicyShadow{
					{
						PolicyName: overridePolicy1.Name,
						Overriders: overriders1,
					},
					{
						PolicyName: overridePolicy2.Name,
						Overriders: overriders2,
					},
				},
			},
			wantedCOPs: &AppliedOverrides{
				AppliedItems: []OverridePolicyShadow{
					{
						PolicyName: overridePolicy3.Name,
						Overriders: overriders3,
					},
				},
			},
			wantedAnnotations: map[string]string{
				"aaa": "bbb",
				"foo": "bar",
				"cue": "cue",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if cops, ops, err := m.ApplyOverridePolicies(tt.resource, tt.operation); !reflect.DeepEqual(cops, tt.wantedCOPs) || !reflect.DeepEqual(ops, tt.wantedOPs) ||
				!reflect.DeepEqual(tt.resource.GetAnnotations(), tt.wantedAnnotations) || !reflect.DeepEqual(err, tt.wantedErr) {
				t.Errorf("ApplyOverridePolicies() = %v, %v, %v, want %v, %v, %v", cops, ops, err, tt.wantedCOPs, tt.wantedOPs, tt.wantedErr)
			}
		})
	}
}
