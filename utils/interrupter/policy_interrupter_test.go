package interrupter

import (
	"reflect"
	"testing"

	jsonpatchv2 "gomodules.xyz/jsonpatch/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/utils/templatemanager"
	"github.com/k-cloud-labs/pkg/utils/templatemanager/templates"
	"github.com/k-cloud-labs/pkg/utils/tokenmanager"
)

func Test_policyInterrupterImpl_renderAndFormat(t *testing.T) {
	mtm, err := templatemanager.NewOverrideTemplateManager(&templatemanager.TemplateSource{
		Content:      templates.OverrideTemplate,
		TemplateName: "BaseTemplate",
	})
	if err != nil {
		t.Error(err)
		return
	}

	vtm, err := templatemanager.NewValidateTemplateManager(&templatemanager.TemplateSource{
		Content:      templates.ValidateTemplate,
		TemplateName: "BaseTemplate",
	})
	if err != nil {
		t.Error(err)
		return
	}

	intPtr := func(i int64) *int64 {
		return &i
	}

	policyInterrupter := &policyInterrupterImpl{
		overrideTemplateManager: mtm,
		validateTemplateManager: vtm,
		cueManager:              templatemanager.NewCueManager(),
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
			name: "validatePAB",
			args: args{
				data: &policyv1alpha1.ValidateRuleTemplate{
					Type: policyv1alpha1.ValidateRuleTypePodAvailableBadge,
					PodAvailableBadge: &policyv1alpha1.PodAvailableBadge{
						MaxUnavailable: &intstr.IntOrString{
							Type:   intstr.String,
							StrVal: "60%",
						},
						ReplicaReference: &policyv1alpha1.ReplicaResourceRefer{
							From:               policyv1alpha1.FromOwnerReference,
							TargetReplicaPath:  "/spec/replica",
							CurrentReplicaPath: "/status/replica",
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

			gotB, err := policyInterrupter.renderAndFormat(tt.args.data)
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

func Test_applyJSONPatch(t *testing.T) {
	type args struct {
		obj       *unstructured.Unstructured
		overrides []jsonpatchv2.JsonPatchOperation
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "1",
			args: args{
				obj: &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "OverridePolicy",
				}},
				overrides: []jsonpatchv2.JsonPatchOperation{
					{
						Operation: "add",
						Path:      "/kind",
						Value:     "ClusterOverridePolicy",
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := applyJSONPatch(tt.args.obj, tt.args.overrides); (err != nil) != tt.wantErr {
				t.Errorf("applyJSONPatch() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_policyInterrupterImpl_OnValidating(t *testing.T) {
	mtm, err := templatemanager.NewOverrideTemplateManager(&templatemanager.TemplateSource{
		Content:      templates.OverrideTemplate,
		TemplateName: "BaseTemplate",
	})
	if err != nil {
		t.Error(err)
		return
	}

	vtm, err := templatemanager.NewValidateTemplateManager(&templatemanager.TemplateSource{
		Content:      templates.ValidateTemplate,
		TemplateName: "BaseTemplate",
	})
	if err != nil {
		t.Error(err)
		return
	}

	policyInterrupter := NewPolicyInterrupterManager(mtm, vtm, templatemanager.NewCueManager(), tokenmanager.NewTokenManager())

	type args struct {
		obj    *unstructured.Unstructured
		oldObj *unstructured.Unstructured
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "1",
			args: args{
				obj: &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "OverridePolicy",
					"spec": map[string]interface{}{
						"overrideRules": []map[string]interface{}{
							{
								"overriders": map[string]interface{}{
									"renderedCue": `
 data:      _ @tag(data)
object:    data.object
oldObject: data.oldObject
validate: {
	if object.metadata.annotations."no-delete" != _|_ {
		valid:  false
		reason: "cannot delete this ns"
	}
}
`,
								},
							},
						},
					},
				}},
			},
			wantErr: false,
		},
		{
			name: "2",
			args: args{
				obj: &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "ClusterOverridePolicy",
					"spec": map[string]interface{}{
						"overrideRules": []map[string]interface{}{
							{
								"overriders": map[string]interface{}{
									"renderedCue": `
 data:      _ @tag(data)
object:    data.object
oldObject: data.oldObject
validate: {
	if object.metadata.annotations."no-delete" != _|_ {
		valid:  false
		reason: "cannot delete this ns"
	}
}
`,
								},
							},
						},
					},
				}},
			},
			wantErr: false,
		},
		{
			name: "3",
			args: args{
				obj: &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "ClusterValidatePolicy",
					"spec": map[string]interface{}{
						"validateRules": []map[string]interface{}{
							{
								"renderedCue": `
 data:      _ @tag(data)
object:    data.object
oldObject: data.oldObject
validate: {
	if object.metadata.annotations."no-delete" != _|_ {
		valid:  false
		reason: "cannot delete this ns"
	}
}
`,
							},
						},
					},
				}},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := policyInterrupter.OnValidating(tt.args.obj, tt.args.oldObj); (err != nil) != tt.wantErr {
				t.Errorf("OnValidating() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_policyInterrupterImpl_OnMutating(t *testing.T) {
	mtm, err := templatemanager.NewOverrideTemplateManager(&templatemanager.TemplateSource{
		Content:      templates.OverrideTemplate,
		TemplateName: "BaseTemplate",
	})
	if err != nil {
		t.Error(err)
		return
	}

	vtm, err := templatemanager.NewValidateTemplateManager(&templatemanager.TemplateSource{
		Content:      templates.ValidateTemplate,
		TemplateName: "BaseTemplate",
	})
	if err != nil {
		t.Error(err)
		return
	}

	policyInterrupter := NewPolicyInterrupterManager(mtm, vtm, templatemanager.NewCueManager(), tokenmanager.NewTokenManager())

	type args struct {
		obj    *unstructured.Unstructured
		oldObj *unstructured.Unstructured
	}
	tests := []struct {
		name    string
		args    args
		want    []jsonpatchv2.JsonPatchOperation
		wantErr bool
	}{
		{
			name: "1",
			args: args{
				obj: &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "OverridePolicy",
					"spec": map[string]interface{}{
						"overrideRules": []map[string]interface{}{
							{
								"overriders": map[string]interface{}{
									"template": map[string]interface{}{
										"type":      "annotations",
										"operation": "replace",
										"path":      "add-by",
										"value": map[string]interface{}{
											"string": "cue",
										},
									},
								},
							},
						},
					},
				}},
			},
			want: []jsonpatchv2.JsonPatchOperation{
				{
					Operation: "replace",
					Path:      "/spec/overrideRules/0/overriders/renderedCue",
					Value: `import (
    "strings"
    "strconv"
    "math"
    "list"
)

data:      _ @tag(data)
object:    data.object
kind:      object.kind
oldObject: data.oldObject
unFlattenPatches: [
    if object.metadata.annotations == _|_ {
        {
            op:   "replace"
            path: "/metadata/annotations"
            value: {}
        }
    },
    // annotations
    {
        op:    "replace"
        path:  "/metadata/annotations/add-by"
        value: "cue"
    },
]
patches: list.FlattenN(unFlattenPatches, -1)
`,
				},
			},
			wantErr: false,
		},
		{
			name: "2",
			args: args{
				obj: &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "ClusterOverridePolicy",
					"spec": map[string]interface{}{
						"overrideRules": []map[string]interface{}{
							{
								"overriders": map[string]interface{}{
									"template": map[string]interface{}{
										"type":      "annotations",
										"operation": "replace",
										"path":      "add-by",
										"value": map[string]interface{}{
											"string": "cue",
										},
									},
								},
							},
						},
					},
				}},
			},
			want: []jsonpatchv2.JsonPatchOperation{
				{
					Operation: "replace",
					Path:      "/spec/overrideRules/0/overriders/renderedCue",
					Value: `import (
    "strings"
    "strconv"
    "math"
    "list"
)

data:      _ @tag(data)
object:    data.object
kind:      object.kind
oldObject: data.oldObject
unFlattenPatches: [
    if object.metadata.annotations == _|_ {
        {
            op:   "replace"
            path: "/metadata/annotations"
            value: {}
        }
    },
    // annotations
    {
        op:    "replace"
        path:  "/metadata/annotations/add-by"
        value: "cue"
    },
]
patches: list.FlattenN(unFlattenPatches, -1)
`,
				},
			},
			wantErr: false,
		},
		{
			name: "3",
			args: args{
				obj: &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "ClusterValidatePolicy",
					"spec": map[string]interface{}{
						"validateRules": []map[string]interface{}{
							{
								"template": map[string]interface{}{
									"type": "condition",
									"condition": map[string]interface{}{
										"message": "forbidden",
										"cond":    "Gte",
										"dataRef": map[string]interface{}{
											"from": "current",
											"path": "/spec/replica",
										},
										"value": map[string]interface{}{
											"integer": 1,
										},
									},
								},
							},
						},
					},
				}},
			},
			want: []jsonpatchv2.JsonPatchOperation{
				{
					Operation: "replace",
					Path:      "/spec/validateRules/0/template/condition/affectMode",
					Value:     "reject",
				},
				{
					Operation: "replace",
					Path:      "/spec/validateRules/0/renderedCue",
					Value: `data:      _ @tag(data)
object:    data.object
oldObject: data.oldObject
validate: {
    if object.spec.replica != _|_ {
        if object.spec.replica >= 1 {
            valid:  false
            reason: "forbidden"
        }
    }
}
`,
				},
			},
			wantErr: false,
		},
		{
			name: "4",
			args: args{
				obj: &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "ClusterValidatePolicy",
					"spec": map[string]interface{}{
						"validateRules": []map[string]interface{}{
							{
								"template": map[string]interface{}{
									"type": "pab",
									"podAvailableBadge": map[string]interface{}{
										"maxUnavailable": "60%",
									},
								},
							},
						},
					},
				}},
			},
			want: []jsonpatchv2.JsonPatchOperation{
				{
					Operation: "replace",
					Path:      "/spec/validateRules/0/template/podAvailableBadge/replicaReference",
					Value: &policyv1alpha1.ReplicaResourceRefer{
						From:               policyv1alpha1.FromOwnerReference,
						TargetReplicaPath:  "/spec/replicas",
						CurrentReplicaPath: "/status/replicas",
					},
				},
				{
					Operation: "replace",
					Path:      "/spec/validateRules/0/renderedCue",
					Value: `data:        _ @tag(data)
object:      data.object
oldObject:   data.oldObject
otherObject: data.extraParams."otherObject"
validate: {
    if otherObject.spec.replicas != _|_ {
        if otherObject.status.replicas != _|_ {
            // target - target * 0.6 > current
            if otherObject.spec.replicas-otherObject.spec.replicas*0.6 > otherObject.status.replicas-1 {
                {
                    valid:  false
                    reason: "Cannot delete this pod, cause of hitting maxUnavailable(0.6)"
                }
            }
        }
    }
}
`,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got, err := policyInterrupter.OnMutating(tt.args.obj, tt.args.oldObj)
			if (err != nil) != tt.wantErr {
				t.Errorf("OnMutating() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("OnMutating() got = %+v, want %+v", got, tt.want)
			}
		})
	}
}
