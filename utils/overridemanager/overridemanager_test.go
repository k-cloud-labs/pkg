package overridemanager

import (
	"flag"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	admissionv1 "k8s.io/api/admission/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
	v1alpha10 "github.com/k-cloud-labs/pkg/client/listers/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/test/helper"
	"github.com/k-cloud-labs/pkg/test/mock"
	"github.com/k-cloud-labs/pkg/utils"
	"github.com/k-cloud-labs/pkg/utils/cue"
	utilhelper "github.com/k-cloud-labs/pkg/utils/util"
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
					TargetOperations: []admissionv1.Operation{admissionv1.Create},
					Overriders:       overriders1,
				},
				{
					TargetOperations: []admissionv1.Operation{admissionv1.Update},
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
					TargetOperations: []admissionv1.Operation{admissionv1.Create, admissionv1.Update},
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
		operation        admissionv1.Operation
		wantedOverriders []policyOverriders
	}{
		{
			name:      "OverrideRules test 1",
			policies:  []GeneralOverridePolicy{overridePolicy1, overridePolicy2, overridePolicy3},
			resource:  deploymentObj,
			operation: admissionv1.Create,
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
			operation: admissionv1.Update,
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
	stringPtr := func(s string) *string {
		return &s
	}
	overriders4 := policyv1alpha1.Overriders{
		Template: &policyv1alpha1.TemplateRule{
			Type:      policyv1alpha1.RuleTypeAnnotations,
			Operation: policyv1alpha1.OverriderOpReplace,
			Path:      "owned-by",
			Value:     &policyv1alpha1.CustomTypes{String: stringPtr("template-cue")},
		},
		RenderedCue: `
data:      _ @tag(data)
object:    data.object
kind:      object.kind
patches: [
    if object.metadata.annotations == _|_ {
        {
            op:   "replace"
            path: "/metadata/annotations"
            value: {}
        }
    },
    // annotations
	{
		op:   "replace"
		path: "/metadata/annotations/owned-by"
		value: "template-cue"
	}
]
`,
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
					TargetOperations: []admissionv1.Operation{admissionv1.Create},
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
					TargetOperations: []admissionv1.Operation{admissionv1.Create},
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
					TargetOperations: []admissionv1.Operation{admissionv1.Create, admissionv1.Update},
					Overriders:       overriders3,
				},
			},
		},
	}
	overridePolicy4 := &policyv1alpha1.OverridePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceDefault,
			Name:      "overridePolicy4",
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
					TargetOperations: []admissionv1.Operation{admissionv1.Create},
					Overriders:       overriders4,
				},
			},
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	opLister := mock.NewMockOverridePolicyLister(ctrl)
	copLister := mock.NewMockClusterOverridePolicyLister(ctrl)
	m := NewOverrideManager(nil, copLister, opLister)

	opLister.EXPECT().List(labels.Everything()).Return([]*policyv1alpha1.OverridePolicy{
		overridePolicy1,
		overridePolicy2,
		overridePolicy4,
	}, nil).AnyTimes()

	copLister.EXPECT().List(labels.Everything()).Return([]*policyv1alpha1.ClusterOverridePolicy{
		overridePolicy3,
	}, nil).AnyTimes()

	tests := []struct {
		name              string
		opLister          v1alpha10.OverridePolicyLister
		copLister         v1alpha10.ClusterOverridePolicyLister
		resource          *unstructured.Unstructured
		oldResource       *unstructured.Unstructured
		operation         admissionv1.Operation
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
			operation: admissionv1.Create,
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
					{
						PolicyName: overridePolicy4.Name,
						Overriders: overriders4,
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
				"aaa":      "bbb",
				"foo":      "bar",
				"cue":      "cue",
				"owned-by": "template-cue",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if cops, ops, err := m.ApplyOverridePolicies(tt.resource, tt.oldResource, tt.operation); !reflect.DeepEqual(cops, tt.wantedCOPs) || !reflect.DeepEqual(ops, tt.wantedOPs) ||
				!reflect.DeepEqual(tt.resource.GetAnnotations(), tt.wantedAnnotations) || !reflect.DeepEqual(err, tt.wantedErr) {
				t.Errorf("ApplyOverridePolicies(), cops= %v\n ops=%v\n, err=%v\n, want cops= %v\n ops=%v\n, err=%v", cops, ops, err, tt.wantedCOPs, tt.wantedOPs, tt.wantedErr)
			}
		})
	}
}

func Test_executeCueV2(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ExitOnError)
	klog.InitFlags(fs)
	if err := fs.Parse([]string{"-v", "4"}); err != nil {
		t.Errorf("parse flag err=%v", err)
		return
	}

	type args struct {
		cueStr     string
		parameters []cue.Parameter
	}
	tests := []struct {
		name    string
		args    args
		want    []overrideOption
		wantErr bool
	}{
		{
			name: "normal",
			args: args{
				cueStr: `
data: _ @tag(data)
patches: [
	{
		op: "add"
		path: "abc"
		value: data.object.name
	}
]
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
			want: []overrideOption{
				{
					Op:    "add",
					Path:  "abc",
					Value: "cue",
				},
			},
			wantErr: false,
		},
		{
			name: "error",
			args: args{
				cueStr: `
data: _ @tag(data)
patches: [
	{
		op: "add"
		path: "abc"
		value: data.object.name2
	}
]
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
			wantErr: true,
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
