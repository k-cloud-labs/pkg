package interrupter

import (
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	jsonpatchv2 "gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/test/mock"
	fakedtokenmanager "github.com/k-cloud-labs/pkg/utils/tokenmanager/fake"
)

func test_fakePolicyInterrupterManager(t *testing.T) (PolicyInterrupterManager, error) {
	// base
	baseInterrupter, err := test_baseInterrupter()
	if err != nil {
		return nil, err
	}

	policyInterrupterManager := NewPolicyInterrupterManager()
	tokenManager := fakedtokenmanager.NewFakeTokenGenerator()

	ctrl := gomock.NewController(t)

	opLister := mock.NewMockOverridePolicyLister(ctrl)
	opLister.EXPECT().List(labels.Everything()).Return([]*policyv1alpha1.OverridePolicy{}, nil).AnyTimes()
	copLister := mock.NewMockClusterOverridePolicyLister(ctrl)
	copLister.EXPECT().List(labels.Everything()).Return([]*policyv1alpha1.ClusterOverridePolicy{}, nil).AnyTimes()
	cvpLister := mock.NewMockClusterValidatePolicyLister(ctrl)
	cvpLister.EXPECT().List(labels.Everything()).Return([]*policyv1alpha1.ClusterValidatePolicy{}, nil).AnyTimes()

	// op
	overridePolicyInterrupter := NewOverridePolicyInterrupter(baseInterrupter, tokenManager, nil, opLister)
	policyInterrupterManager.AddInterrupter(schema.GroupVersionKind{
		Group:   policyv1alpha1.SchemeGroupVersion.Group,
		Version: policyv1alpha1.SchemeGroupVersion.Version,
		Kind:    "OverridePolicy",
	}, overridePolicyInterrupter)
	// cop
	policyInterrupterManager.AddInterrupter(schema.GroupVersionKind{
		Group:   policyv1alpha1.SchemeGroupVersion.Group,
		Version: policyv1alpha1.SchemeGroupVersion.Version,
		Kind:    "ClusterOverridePolicy",
	}, NewClusterOverridePolicyInterrupter(overridePolicyInterrupter, copLister))
	// cvp
	policyInterrupterManager.AddInterrupter(schema.GroupVersionKind{
		Group:   policyv1alpha1.SchemeGroupVersion.Group,
		Version: policyv1alpha1.SchemeGroupVersion.Version,
		Kind:    "ClusterValidatePolicy",
	}, NewClusterValidatePolicyInterrupter(baseInterrupter, tokenManager, nil, cvpLister))

	return policyInterrupterManager, nil
}

func Test_policyInterrupterImpl_OnValidating(t *testing.T) {
	policyInterrupter, err := test_fakePolicyInterrupterManager(t)
	if err != nil {
		t.Error(err)
		return
	}

	type args struct {
		obj       *unstructured.Unstructured
		oldObj    *unstructured.Unstructured
		operation admissionv1.Operation
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "1",
			args: args{
				operation: admissionv1.Update,
				oldObj: &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "OverridePolicy",
					"spec": map[string]any{
						"overrideRules": []map[string]any{
							{
								"overriders": map[string]any{},
							},
						},
					},
				}},
				obj: &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "OverridePolicy",
					"spec": map[string]any{
						"overrideRules": []map[string]any{
							{
								"overriders": map[string]any{
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
}
`,
								},
							},
						},
					},
				}},
			},
			wantErr: true,
		},
		{
			name: "1.1",
			args: args{
				operation: admissionv1.Update,
				oldObj: &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "OverridePolicy",
					"spec": map[string]any{
						"overrideRules": []map[string]any{
							{
								"overriders": map[string]any{
									"renderedCue": ``,
								},
							},
						},
					},
				}},
				obj: &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "OverridePolicy",
					"spec": map[string]any{
						"overrideRules": []map[string]any{
							{
								"overriders": map[string]any{
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
			name: "1.3",
			args: args{
				operation: admissionv1.Delete,
				obj: &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "OverridePolicy",
				},
				},
			},
			wantErr: false,
		},
		{
			name: "2",
			args: args{
				operation: admissionv1.Update,
				oldObj: &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "ClusterOverridePolicy",
					"spec": map[string]any{
						"overrideRules": []map[string]any{
							{
								"overriders": map[string]any{
									"renderedCue": ``,
								},
							},
						},
					},
				}},
				obj: &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "ClusterOverridePolicy",
					"spec": map[string]any{
						"overrideRules": []map[string]any{
							{
								"overriders": map[string]any{
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
			name: "2.1",
			args: args{
				operation: admissionv1.Create,
				obj: &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "ClusterOverridePolicy",
					"spec": map[string]any{
						"overrideRules": []map[string]any{
							{
								"overriders": map[string]any{
									"renderedCue": `
 data:      _ @tag(data)
object:    data.object
oldObject: data.oldObject
validate: {
	if object.metadata.annotations."no-delete" != _|_ {
		valid:  false
		reason: "cannot delete this ns"
	} invalid cue here
}
`,
								},
							},
						},
					},
				}},
			},
			wantErr: true,
		},
		{
			name: "2.3",
			args: args{
				operation: admissionv1.Delete,
				obj: &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "ClusterOverridePolicy",
				},
				},
			},
			wantErr: false,
		},
		{
			name: "3",
			args: args{
				operation: admissionv1.Update,
				oldObj: &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "ClusterValidatePolicy",
					"spec": map[string]any{
						"validateRules": []map[string]any{
							{
								"renderedCue": ``,
							},
						},
					},
				}},
				obj: &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "ClusterValidatePolicy",
					"spec": map[string]any{
						"validateRules": []map[string]any{
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
		{
			name: "3.1",
			args: args{
				operation: admissionv1.Create,
				obj: &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "ClusterValidatePolicy",
					"spec": map[string]any{
						"validateRules": []map[string]any{
							{
								"renderedCue": `
 data:      _ @tag(data)
object:    data.object
oldObject: data.oldObject
validate: {
	if object.metadata.annotations."no-delete" != _|_ {
		valid:  false
		reason: "cannot delete this ns"
	} invalid cue here
}
`,
							},
						},
					},
				}},
			},
			wantErr: true,
		},
		{
			name: "3.3",
			args: args{
				operation: admissionv1.Delete,
				obj: &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "ClusterValidatePolicy",
				},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := policyInterrupter.OnValidating(tt.args.obj, tt.args.oldObj, tt.args.operation); (err != nil) != tt.wantErr {
				t.Errorf("OnValidating() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_policyInterrupterImpl_OnMutating(t *testing.T) {
	policyInterrupter, err := test_fakePolicyInterrupterManager(t)
	if err != nil {
		t.Error(err)
		return
	}

	type args struct {
		obj       *unstructured.Unstructured
		oldObj    *unstructured.Unstructured
		operation admissionv1.Operation
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
				operation: admissionv1.Create,
				obj: &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "OverridePolicy",
					"spec": map[string]any{
						"overrideRules": []map[string]any{
							{
								"overriders": map[string]any{
									"template": map[string]any{
										"type":      "annotations",
										"operation": "replace",
										"path":      "add-by",
										"value": map[string]any{
											"string": "cue",
										},
									},
								},
							},
							{
								"overriders": map[string]any{
									"template": map[string]any{
										"type":      "annotations",
										"operation": "replace",
										"path":      "add-by",
										"valueRef": map[string]any{
											"from": "http",
											"path": "body.name",
											"http": map[string]any{
												"url": "https://xxx.com",
												"auth": map[string]any{
													"authUrl":  "https://xxx.com",
													"username": "xx",
													"password": "xx",
												},
											},
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
				{
					Operation: "replace",
					Path:      "/spec/overrideRules/1/overriders/renderedCue",
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
http:      data.extraParams."http"
unFlattenPatches: [
    if object.metadata.annotations == _|_ {
        {
            op:   "replace"
            path: "/metadata/annotations"
            value: {}
        }
    },
    // annotations
    if http.body.name != _|_ {
        {
            op:    "replace"
            path:  "/metadata/annotations/add-by"
            value: http.body.name
        }
    },
]
patches: list.FlattenN(unFlattenPatches, -1)
`,
				},
			},
			wantErr: false,
		},
		{
			name: "1.1",
			args: args{
				operation: admissionv1.Delete,
				obj: &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "OverridePolicy",
					"spec": map[string]any{
						"overrideRules": []map[string]any{
							{
								"overriders": map[string]any{
									"template": map[string]any{
										"type":      "annotations",
										"operation": "replace",
										"path":      "add-by",
										"value": map[string]any{
											"string": "cue",
										},
									},
								},
							},
							{
								"overriders": map[string]any{
									"template": map[string]any{
										"type":      "annotations",
										"operation": "replace",
										"path":      "add-by",
										"valueRef": map[string]any{
											"from": "http",
											"path": "body.name",
											"http": map[string]any{
												"url": "https://xxx.com",
												"auth": map[string]any{
													"authUrl":  "https://xxx.com",
													"username": "xx",
													"password": "xx",
												},
											},
										},
									},
								},
							},
						},
					},
				}},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "1.2",
			args: args{
				operation: admissionv1.Update,
				oldObj: &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "OverridePolicy",
					"spec": map[string]any{
						"overrideRules": []map[string]any{
							{
								"overriders": map[string]any{
									"template": map[string]any{
										"type":      "annotations",
										"operation": "replace",
										"path":      "add-by",
										"value": map[string]any{
											"string": "cue",
										},
									},
								},
							},
							{
								"overriders": map[string]any{
									"template": map[string]any{
										"type":      "annotations",
										"operation": "replace",
										"path":      "add-by",
										"valueRef": map[string]any{
											"from": "http",
											"path": "body.name",
											"http": map[string]any{
												"url": "https://xxx.com",
												"auth": map[string]any{
													"authUrl":  "https://xxx.com",
													"username": "xx",
													"password": "xx",
												},
											},
										},
									},
								},
							},
						},
					},
				}},
				obj: &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "OverridePolicy",
					"spec": map[string]any{
						"overrideRules": []map[string]any{
							{
								"overriders": map[string]any{
									"template": map[string]any{
										"type":      "annotations",
										"operation": "replace",
										"path":      "add-by",
										"value": map[string]any{
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
				operation: admissionv1.Create,
				obj: &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "ClusterOverridePolicy",
					"spec": map[string]any{
						"overrideRules": []map[string]any{
							{
								"overriders": map[string]any{
									"template": map[string]any{
										"type":      "annotations",
										"operation": "replace",
										"path":      "add-by",
										"value": map[string]any{
											"string": "cue",
										},
									},
								},
							},
							{
								"overriders": map[string]any{
									"template": map[string]any{
										"type":      "annotations",
										"operation": "replace",
										"path":      "add-by",
										"valueRef": map[string]any{
											"from": "http",
											"path": "body.name",
											"http": map[string]any{
												"url": "https://xxx.com",
												"auth": map[string]any{
													"authUrl":  "https://xxx.com",
													"username": "xx",
													"password": "xx",
												},
											},
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
				{
					Operation: "replace",
					Path:      "/spec/overrideRules/1/overriders/renderedCue",
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
http:      data.extraParams."http"
unFlattenPatches: [
    if object.metadata.annotations == _|_ {
        {
            op:   "replace"
            path: "/metadata/annotations"
            value: {}
        }
    },
    // annotations
    if http.body.name != _|_ {
        {
            op:    "replace"
            path:  "/metadata/annotations/add-by"
            value: http.body.name
        }
    },
]
patches: list.FlattenN(unFlattenPatches, -1)
`,
				},
			},
			wantErr: false,
		},
		{
			name: "2.1",
			args: args{
				operation: admissionv1.Delete,
				obj: &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "ClusterOverridePolicy",
					"spec": map[string]any{
						"overrideRules": []map[string]any{
							{
								"overriders": map[string]any{
									"template": map[string]any{
										"type":      "annotations",
										"operation": "replace",
										"path":      "add-by",
										"value": map[string]any{
											"string": "cue",
										},
									},
								},
							},
							{
								"overriders": map[string]any{
									"template": map[string]any{
										"type":      "annotations",
										"operation": "replace",
										"path":      "add-by",
										"valueRef": map[string]any{
											"from": "http",
											"path": "body.name",
											"http": map[string]any{
												"url": "https://xxx.com",
												"auth": map[string]any{
													"authUrl":  "https://xxx.com",
													"username": "xx",
													"password": "xx",
												},
											},
										},
									},
								},
							},
						},
					},
				}},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "2.2",
			args: args{
				operation: admissionv1.Update,
				oldObj: &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "ClusterOverridePolicy",
					"spec": map[string]any{
						"overrideRules": []map[string]any{
							{
								"overriders": map[string]any{
									"template": map[string]any{
										"type":      "annotations",
										"operation": "replace",
										"path":      "add-by",
										"value": map[string]any{
											"string": "cue",
										},
									},
								},
							},
							{
								"overriders": map[string]any{
									"template": map[string]any{
										"type":      "annotations",
										"operation": "replace",
										"path":      "add-by",
										"valueRef": map[string]any{
											"from": "http",
											"path": "body.name",
											"http": map[string]any{
												"url": "https://xxx.com",
												"auth": map[string]any{
													"authUrl":  "https://xxx.com",
													"username": "xx",
													"password": "xx",
												},
											},
										},
									},
								},
							},
						},
					},
				}},
				obj: &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "ClusterOverridePolicy",
					"spec": map[string]any{
						"overrideRules": []map[string]any{
							{
								"overriders": map[string]any{
									"template": map[string]any{
										"type":      "annotations",
										"operation": "replace",
										"path":      "add-by",
										"value": map[string]any{
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
				operation: admissionv1.Create,
				obj: &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "ClusterValidatePolicy",
					"spec": map[string]any{
						"validateRules": []map[string]any{
							{
								"template": map[string]any{
									"type": "condition",
									"condition": map[string]any{
										"message": "forbidden",
										"cond":    "Gte",
										"dataRef": map[string]any{
											"from": "current",
											"path": "/spec/replica",
										},
										"value": map[string]any{
											"integer": 1,
										},
									},
								},
							},
							{
								"template": map[string]any{
									"type": "condition",
									"condition": map[string]any{
										"message": "forbidden",
										"cond":    "Equal",
										"dataRef": map[string]any{
											"from": "http",
											"path": "body.origin",
											"http": map[string]any{
												"url": "https://xxx.com",
												"auth": map[string]any{
													"authUrl":  "https://xxx.com",
													"username": "xx",
													"password": "xx",
												},
											},
										},
										"valueRef": map[string]any{
											"from": "http",
											"path": "body.target",
											"http": map[string]any{
												"url": "https://xxx.com",
												"auth": map[string]any{
													"authUrl":  "https://xxx.com",
													"username": "xx",
													"password": "xx",
												},
											},
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
				{
					Operation: "replace",
					Path:      "/spec/validateRules/1/template/condition/affectMode",
					Value:     "reject",
				},
				{
					Operation: "replace",
					Path:      "/spec/validateRules/1/renderedCue",
					Value: `data:      _ @tag(data)
object:    data.object
oldObject: data.oldObject
http:      data.extraParams."http"
http_d:    data.extraParams."http_d"
validate: {
    if http_d.body.origin != _|_ {
        if http.body.target != _|_ {
            if http_d.body.origin == http.body.target {
                valid:  false
                reason: "forbidden"
            }
        }
    }
}
`,
				},
			},
			wantErr: false,
		},
		{
			name: "3.0",
			args: args{
				operation: admissionv1.Create,
				obj: &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "ClusterValidatePolicy",
					"spec": map[string]any{
						"validateRules": []map[string]any{
							{
								"template": map[string]any{
									"type": "condition",
									"condition": map[string]any{
										"message":    "forbidden",
										"cond":       "NotIn",
										"affectMode": "reject",
										"dataRef": map[string]any{
											"from": "current",
											"path": "/spec/containers/0/image",
										},
										"value": map[string]any{
											"stringSlice": []string{"fake-image", "fake-image-v2"},
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
					Path:      "/spec/validateRules/0/renderedCue",
					Value: `import "list"

data:      _ @tag(data)
object:    data.object
oldObject: data.oldObject
validate: {
    if object.spec.containers[0].image != _|_ {
        if !list.Contains(["fake-image", "fake-image-v2"], object.spec.containers[0].image) {
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
			name: "3.1",
			args: args{
				operation: admissionv1.Delete,
				obj: &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "ClusterValidatePolicy",
					"spec": map[string]any{
						"validateRules": []map[string]any{
							{
								"template": map[string]any{
									"type": "condition",
									"condition": map[string]any{
										"message": "forbidden",
										"cond":    "Gte",
										"dataRef": map[string]any{
											"from": "current",
											"path": "/spec/replica",
										},
										"value": map[string]any{
											"integer": 1,
										},
									},
								},
							},
							{
								"template": map[string]any{
									"type": "condition",
									"condition": map[string]any{
										"message": "forbidden",
										"cond":    "Equal",
										"dataRef": map[string]any{
											"from": "http",
											"path": "body.origin",
											"http": map[string]any{
												"url": "https://xxx.com",
												"auth": map[string]any{
													"authUrl":  "https://xxx.com",
													"username": "xx",
													"password": "xx",
												},
											},
										},
										"valueRef": map[string]any{
											"from": "http",
											"path": "body.target",
											"http": map[string]any{
												"url": "https://xxx.com",
												"auth": map[string]any{
													"authUrl":  "https://xxx.com",
													"username": "xx",
													"password": "xx",
												},
											},
										},
									},
								},
							},
						},
					},
				}},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "3.2",
			args: args{
				operation: admissionv1.Update,
				oldObj: &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "ClusterValidatePolicy",
					"spec": map[string]any{
						"validateRules": []map[string]any{
							{
								"template": map[string]any{
									"type": "condition",
									"condition": map[string]any{
										"message": "forbidden",
										"cond":    "Gte",
										"dataRef": map[string]any{
											"from": "current",
											"path": "/spec/replica",
										},
										"value": map[string]any{
											"integer": 1,
										},
									},
								},
							},
							{
								"template": map[string]any{
									"type": "condition",
									"condition": map[string]any{
										"message": "forbidden",
										"cond":    "Equal",
										"dataRef": map[string]any{
											"from": "http",
											"path": "body.origin",
											"http": map[string]any{
												"url": "https://xxx.com",
												"auth": map[string]any{
													"authUrl":  "https://xxx.com",
													"username": "xx",
													"password": "xx",
												},
											},
										},
										"valueRef": map[string]any{
											"from": "http",
											"path": "body.target",
											"http": map[string]any{
												"url": "https://xxx.com",
												"auth": map[string]any{
													"authUrl":  "https://xxx.com",
													"username": "xx",
													"password": "xx",
												},
											},
										},
									},
								},
							},
						},
					},
				}},
				obj: &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "ClusterValidatePolicy",
					"spec": map[string]any{
						"validateRules": []map[string]any{
							{
								"template": map[string]any{
									"type": "condition",
									"condition": map[string]any{
										"message": "forbidden",
										"cond":    "Gte",
										"dataRef": map[string]any{
											"from": "current",
											"path": "/spec/replica",
										},
										"value": map[string]any{
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got, err := policyInterrupter.OnMutating(tt.args.obj, tt.args.oldObj, tt.args.operation)
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

func Test_policyInterrupterImpl_OnStartUp(t *testing.T) {
	policyInterrupter, err := test_fakePolicyInterrupterManager(t)
	if err != nil {
		t.Error(err)
		return
	}

	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "1",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := policyInterrupter.OnStartUp(); (err != nil) != tt.wantErr {
				t.Errorf("OnStartUp() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
