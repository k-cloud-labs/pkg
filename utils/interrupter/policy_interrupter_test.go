package interrupter

import (
	"reflect"
	"testing"

	jsonpatchv2 "gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
	fakedtokenmanager "github.com/k-cloud-labs/pkg/utils/tokenmanager/fake"
)

func test_fakePolicyInterrupterManager() (PolicyInterrupterManager, error) {
	// base
	baseInterrupter, err := test_baseInterrupter()
	if err != nil {
		return nil, err
	}

	policyInterrupterManager := NewPolicyInterrupterManager()
	tokenManager := fakedtokenmanager.NewFakeTokenGenerator()

	// op
	overridePolicyInterrupter := NewOverridePolicyInterrupter(baseInterrupter, tokenManager, nil, nil)
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
	}, NewClusterOverridePolicyInterrupter(overridePolicyInterrupter, nil))
	// cvp
	policyInterrupterManager.AddInterrupter(schema.GroupVersionKind{
		Group:   policyv1alpha1.SchemeGroupVersion.Group,
		Version: policyv1alpha1.SchemeGroupVersion.Version,
		Kind:    "ClusterValidatePolicy",
	}, NewClusterValidatePolicyInterrupter(baseInterrupter, tokenManager, nil, nil))

	return policyInterrupterManager, nil
}

func Test_policyInterrupterImpl_OnValidating(t *testing.T) {
	policyInterrupter, err := test_fakePolicyInterrupterManager()
	if err != nil {
		t.Error(err)
		return
	}

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
			name: "0",
			args: args{
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
			name: "1",
			args: args{
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
			name: "2",
			args: args{
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
			name: "3",
			args: args{
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := policyInterrupter.OnValidating(tt.args.obj, tt.args.oldObj, admissionv1.Create); (err != nil) != tt.wantErr {
				t.Errorf("OnValidating() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_policyInterrupterImpl_OnMutating(t *testing.T) {
	policyInterrupter, err := test_fakePolicyInterrupterManager()
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
			name: "4",
			args: args{
				operation: admissionv1.Create,
				obj: &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "ClusterValidatePolicy",
					"spec": map[string]any{
						"validateRules": []map[string]any{
							{
								"template": map[string]any{
									"type": "pab",
									"podAvailableBadge": map[string]any{
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
		{
			name: "4.1",
			args: args{
				operation: admissionv1.Delete,
				obj: &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "ClusterValidatePolicy",
					"spec": map[string]any{
						"validateRules": []map[string]any{
							{
								"template": map[string]any{
									"type": "pab",
									"podAvailableBadge": map[string]any{
										"maxUnavailable": "60%",
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
			name: "5",
			args: args{
				operation: admissionv1.Create,
				obj: &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "ClusterValidatePolicy",
					"spec": map[string]any{
						"validateRules": []map[string]any{
							{
								"template": map[string]any{
									"type": "pab",
									"podAvailableBadge": map[string]any{
										"maxUnavailable": "60%",
										"replicaReference": map[string]any{
											"from":               "http",
											"targetReplicaPath":  "body.target",
											"currentReplicaPath": "body.origin",
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
					Path:      "/spec/validateRules/0/renderedCue",
					Value: `data:      _ @tag(data)
object:    data.object
oldObject: data.oldObject
http:      data.extraParams."http"
validate: {
    if http.body.target != _|_ {
        if http.body.origin != _|_ {
            // target - target * 0.6 > current
            if http.body.target-http.body.target*0.6 > http.body.origin-1 {
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
		{
			name: "5.1",
			args: args{
				operation: admissionv1.Delete,
				obj: &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "policy.kcloudlabs.io/v1alpha1",
					"kind":       "ClusterValidatePolicy",
					"spec": map[string]any{
						"validateRules": []map[string]any{
							{
								"template": map[string]any{
									"type": "pab",
									"podAvailableBadge": map[string]any{
										"maxUnavailable": "60%",
										"replicaReference": map[string]any{
											"from":               "http",
											"targetReplicaPath":  "body.target",
											"currentReplicaPath": "body.origin",
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
