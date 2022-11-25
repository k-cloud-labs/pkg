package cue

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/utils/dynamiclister"
	fakedl "github.com/k-cloud-labs/pkg/utils/dynamiclister/fake"
)

func newEmptyObj() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{}}
}

func newBasicObj(name, ns string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{}}
	obj.SetName(name)
	obj.SetNamespace(ns)

	return obj
}

func Test_parseAndGetRefValue(t *testing.T) {
	type args struct {
		refKey string
		keys   []string
		value  any
	}
	tests := []struct {
		name     string
		args     args
		want     string
		wantBool bool
		wantErr  bool
	}{
		{
			name: "normal",
			args: args{
				refKey: "{{metadata.name}}",
				keys:   []string{"metadata", "name"},
				value:  "name1",
			},
			want:     "name1",
			wantBool: true,
			wantErr:  false,
		},
		{
			name: "normal2",
			args: args{
				refKey: "metadata.name",
				keys:   []string{"metadata", "namespace"},
				value:  "ns",
			},
			want:     "metadata.name",
			wantBool: true,
			wantErr:  false,
		},
		{
			name: "normal3",
			args: args{
				refKey: "https://xxxx.com/{{metadata.namespace}}",
				keys:   []string{"metadata", "namespace"},
				value:  "ns",
			},
			want:     "https://xxxx.com/ns",
			wantBool: true,
			wantErr:  false,
		},
		{
			name: "error",
			args: args{
				refKey: "{{metadata.name}}",
				keys:   []string{"metadata", "namespace"},
				value:  "ns",
			},
			wantBool: false,
			wantErr:  false,
		},
		{
			name: "error2",
			args: args{
				refKey: "{{metadata.name.name}}",
				keys:   []string{"metadata", "namespace"},
				value:  "ns",
			},
			wantErr:  false,
			wantBool: false,
		},
		{
			name: "error3",
			args: args{
				refKey: "{{spec.replica}}",
				keys:   []string{"spec", "replica"},
				value:  int64(1),
			},
			wantErr:  true,
			wantBool: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := newEmptyObj()
			_ = unstructured.SetNestedField(obj.Object, tt.args.value, tt.args.keys...)
			got, gotBool, err := parseAndGetRefValue(tt.args.refKey, obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseAndGetRefValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseAndGetRefValue() got = %v, want %v", got, tt.want)
			}
			if gotBool != tt.wantBool {
				t.Errorf("parseAndGetRefValue() gotBool = %v, want %v", gotBool, tt.wantBool)
			}
		})
	}
}

func Test_handleRefSelectLabels(t *testing.T) {
	type args struct {
		ls  *metav1.LabelSelector
		obj *unstructured.Unstructured
	}
	tests := []struct {
		name     string
		args     args
		want     *metav1.LabelSelector
		wantErr  bool
		wantBool bool
	}{
		{
			name: "label",
			args: args{
				ls: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"name": "{{metadata.name}}",
						"ns":   "{{metadata.namespace}}",
					},
				},
				obj: newBasicObj("name", "ns"),
			},
			want: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"name": "name",
					"ns":   "ns",
				},
				MatchExpressions: make([]metav1.LabelSelectorRequirement, 0),
			},
			wantErr:  false,
			wantBool: true,
		},
		{
			name: "expression",
			args: args{
				ls: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "name",
							Operator: metav1.LabelSelectorOpIn,
							Values: []string{
								"{{metadata.name}}",
								"{{metadata.namespace}}",
							},
						},
					},
				},
				obj: newBasicObj("name", "ns"),
			},
			want: &metav1.LabelSelector{
				MatchLabels: make(map[string]string),
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "name",
						Operator: metav1.LabelSelectorOpIn,
						Values: []string{
							"name",
							"ns",
						},
					},
				},
			},
			wantErr:  false,
			wantBool: true,
		},
		{
			name: "error",
			args: args{
				ls: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"name": "{{metadata.name}}",
						"ns":   "{{metadata.namespace2}}",
					},
				},
				obj: newBasicObj("name", "ns"),
			},
			wantErr:  false,
			wantBool: false,
		},
		{
			name: "error",
			args: args{
				ls: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "name",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{"{{metadata.name2}}"},
						},
					},
				},
				obj: newBasicObj("name", "ns"),
			},
			wantErr:  false,
			wantBool: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotBool, err := handleRefSelectLabels(tt.args.ls, tt.args.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleRefSelectLabels() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("handleRefSelectLabels() got = %v, want %v", got, tt.want)
			}
			if gotBool != tt.wantBool {
				t.Errorf("handleRefSelectLabels() gotBool = %v, want %v", gotBool, tt.wantBool)
			}
		})
	}
}

func Test_getHttpResponse(t *testing.T) {
	s := newMockHttpServer()
	defer s.Close()
	type args struct {
		c   *http.Client
		obj *unstructured.Unstructured
		ref *policyv1alpha1.HttpDataRef
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]any
		wantErr bool
	}{
		{
			name: "normal",
			args: args{
				obj: newBasicObj("name", "ns"),
				ref: &policyv1alpha1.HttpDataRef{
					URL:    "http://127.0.0.1:8090/api/v1/token",
					Method: "GET",
					Params: map[string]string{
						"val": "{{metadata.name}}",
					},
				},
			},
			want: map[string]any{
				"body": map[string]any{
					"token": "name",
				},
			},
			wantErr: false,
		},
		{
			name: "auth1",
			args: args{
				obj: newBasicObj("name", "ns"),
				ref: &policyv1alpha1.HttpDataRef{
					URL:    "http://127.0.0.1:8090/api/v1/token",
					Method: "GET",
					Auth: &policyv1alpha1.HttpRequestAuth{
						StaticToken: "static",
						Token:       "dynamic",
					},
				},
			},
			want: map[string]any{
				// data get from request header authorization
				"body": map[string]any{
					"token": "Bearer static",
				},
			},
			wantErr: false,
		},
		{
			name: "auth2",
			args: args{
				obj: newBasicObj("name", "ns"),
				ref: &policyv1alpha1.HttpDataRef{
					URL:    "http://127.0.0.1:8090/api/v1/token",
					Method: "GET",
					Auth: &policyv1alpha1.HttpRequestAuth{
						Token: "dynamic",
					},
				},
			},
			want: map[string]any{
				// data get from request header authorization
				"body": map[string]any{
					"token": "Bearer dynamic",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getHttpResponse(tt.args.c, tt.args.obj, tt.args.ref)
			if (err != nil) != tt.wantErr {
				t.Errorf("getHttpResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// only compare body since header might different every time test runs
			if !reflect.DeepEqual(got["body"], tt.want["body"]) {
				t.Errorf("getHttpResponse() got = %v, want %v", got["body"], tt.want["body"])
			}
		})
	}
}

func Test_getOwnerReference(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	deploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deploy",
			Namespace: "ns",
		},
	}
	deployObj := newBasicObj("deploy", "ns")
	deployObj.SetAPIVersion("apps/v1")
	deployObj.SetKind("Deployment")
	pod := newBasicObj("pod", "ns")
	pod.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       "deploy",
		},
	})
	dc, err := fakedl.NewFakeDynamicResourceLister(ctx.Done(), deploy)
	if err != nil {
		t.Error(err)
		return
	}
	type args struct {
		c   dynamiclister.DynamicResourceLister
		obj *unstructured.Unstructured
	}
	tests := []struct {
		name    string
		args    args
		want    *unstructured.Unstructured
		wantErr bool
	}{
		{
			name: "pod-deploy",
			args: args{
				c:   dc,
				obj: pod,
			},
			want:    deployObj,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getOwnerReference(tt.args.c, tt.args.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("getOwnerReference() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !equalObj(got, tt.want) {
				t.Errorf("getOwnerReference() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func equalObj(o1, o2 *unstructured.Unstructured) bool {
	if o1.GetAPIVersion() != o2.GetAPIVersion() {
		return false
	}

	if o1.GetNamespace() != o2.GetNamespace() {
		return false
	}

	return o1.GetName() == o2.GetName()
}

func Test_getObject(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	deploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deploy",
			Namespace: "ns",
			Labels: map[string]string{
				"app": "cue",
			},
		},
	}
	deployObj := newBasicObj("deploy", "ns")
	deployObj.SetAPIVersion("apps/v1")
	deployObj.SetKind("Deployment")
	pod := newBasicObj("pod", "ns")
	pod.SetAnnotations(map[string]string{
		"deploy-name": "deploy",
	})
	pod.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       "deploy",
		},
	})
	dc, err := fakedl.NewFakeDynamicResourceLister(ctx.Done(), deploy)
	if err != nil {
		t.Error(err)
		return
	}
	type args struct {
		c   dynamiclister.DynamicResourceLister
		obj *unstructured.Unstructured
		rs  *policyv1alpha1.ResourceSelector
	}
	tests := []struct {
		name    string
		args    args
		want    *unstructured.Unstructured
		wantErr bool
	}{
		{
			name: "deploy_by_name",
			args: args{
				c:   dc,
				obj: pod,
				rs: &policyv1alpha1.ResourceSelector{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Namespace:  "{{metadata.namespace}}",
					Name:       "{{metadata.annotations.deploy-name}}",
				},
			},
			want:    deployObj,
			wantErr: false,
		},
		{
			name: "deploy_by_label",
			args: args{
				c:   dc,
				obj: pod,
				rs: &policyv1alpha1.ResourceSelector{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "cue",
						},
					},
				},
			},
			want:    deployObj,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getObject(tt.args.c, tt.args.obj, tt.args.rs)
			if (err != nil) != tt.wantErr {
				t.Errorf("getObject() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !equalObj(got, tt.want) {
				t.Errorf("getObject() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildCueParamsViaOverridePolicy(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := newMockHttpServer()
	defer s.Close()
	deploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deploy",
			Namespace: "ns",
		},
	}
	deployObj := newBasicObj("deploy", "ns")
	deployObj.SetAPIVersion("apps/v1")
	deployObj.SetKind("Deployment")
	pod := newBasicObj("pod", "ns")
	pod.SetAnnotations(map[string]string{
		"deploy-name": "deploy",
	})
	pod.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       "deploy",
		},
	})
	dc, err := fakedl.NewFakeDynamicResourceLister(ctx.Done(), deploy)
	if err != nil {
		t.Error(err)
		return
	}
	type args struct {
		c         dynamiclister.DynamicResourceLister
		curObject *unstructured.Unstructured
		tmpl      *policyv1alpha1.OverrideRuleTemplate
	}
	tests := []struct {
		name    string
		args    args
		want    *CueParams
		wantErr bool
	}{
		{
			name: "k8s",
			args: args{
				c:         dc,
				curObject: pod,
				tmpl: &policyv1alpha1.OverrideRuleTemplate{
					ValueRef: &policyv1alpha1.ResourceRefer{
						From: policyv1alpha1.FromK8s,
						K8s: &policyv1alpha1.ResourceSelector{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Namespace:  "{{metadata.namespace}}",
							Name:       "{{metadata.annotations.deploy-name}}",
						},
					},
				},
			},
			want: &CueParams{
				ExtraParams: map[string]any{
					"otherObject": deployObj,
				},
			},
			wantErr: false,
		},
		{
			name: "owner",
			args: args{
				c:         dc,
				curObject: pod,
				tmpl: &policyv1alpha1.OverrideRuleTemplate{
					ValueRef: &policyv1alpha1.ResourceRefer{
						From: policyv1alpha1.FromOwnerReference,
					},
				},
			},
			want: &CueParams{
				ExtraParams: map[string]any{
					"otherObject": deployObj,
				},
			},
			wantErr: false,
		},
		{
			name: "http",
			args: args{
				c:         dc,
				curObject: pod,
				tmpl: &policyv1alpha1.OverrideRuleTemplate{
					ValueRef: &policyv1alpha1.ResourceRefer{
						From: policyv1alpha1.FromHTTP,
						Http: &policyv1alpha1.HttpDataRef{
							URL:    "http://127.0.0.1:8090/api/v1/token",
							Method: "GET",
							Params: map[string]string{
								"val": "{{metadata.name}}",
							},
						},
					},
				},
			},
			want: &CueParams{
				ExtraParams: map[string]any{
					"http": map[string]any{
						"body": map[string]any{
							"token": "pod",
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCueParamsViaOverridePolicy(tt.args.c, tt.args.curObject, tt.args.tmpl)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildCueParamsViaOverridePolicy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !equalExtraParams(got.ExtraParams, tt.want.ExtraParams, "") {
				t.Errorf("BuildCueParamsViaOverridePolicy() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func equalExtraParams(e1, e2 map[string]any, keySuffix string) bool {
	if u1, ok := e1["otherObject"+keySuffix]; ok {
		return equalObj(u1.(*unstructured.Unstructured), e2["otherObject"+keySuffix].(*unstructured.Unstructured))
	}

	// http
	h1 := e1["http"+keySuffix]
	b1 := h1.(map[string]any)["body"]

	h2 := e2["http"+keySuffix]
	b2 := h2.(map[string]any)["body"]

	return reflect.DeepEqual(b1, b2)
}

func TestBuildCueParamsViaValidatePolicy(t *testing.T) {
	s := newMockHttpServer()
	defer s.Close()
	deploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deploy",
			Namespace: "ns",
		},
	}
	deployObj := newBasicObj("deploy", "ns")
	deployObj.SetAPIVersion("apps/v1")
	deployObj.SetKind("Deployment")
	pod := newBasicObj("pod", "ns")
	pod.SetAnnotations(map[string]string{
		"deploy-name": "deploy",
	})
	pod.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       "deploy",
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dc, err := fakedl.NewFakeDynamicResourceLister(ctx.Done(), deploy)
	if err != nil {
		t.Error(err)
		return
	}
	type args struct {
		c         dynamiclister.DynamicResourceLister
		curObject *unstructured.Unstructured
		condition *policyv1alpha1.ValidateRuleTemplate
		keySuffix string
	}
	tests := []struct {
		name    string
		args    args
		want    *CueParams
		wantErr bool
	}{
		{
			name: "k8s",
			args: args{
				c:         dc,
				curObject: pod,
				condition: &policyv1alpha1.ValidateRuleTemplate{
					Type: policyv1alpha1.ValidateRuleTypeCondition,
					Condition: &policyv1alpha1.ValidateCondition{
						ValueRef: &policyv1alpha1.ResourceRefer{
							From: policyv1alpha1.FromK8s,
							K8s: &policyv1alpha1.ResourceSelector{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
								Namespace:  "{{metadata.namespace}}",
								Name:       "{{metadata.annotations.deploy-name}}",
							},
						},
					},
				},
			},
			want: &CueParams{
				ExtraParams: map[string]any{
					"otherObject": deployObj,
				},
			},
			wantErr: false,
		},
		{
			name: "owner",
			args: args{
				c:         dc,
				curObject: pod,
				condition: &policyv1alpha1.ValidateRuleTemplate{
					Type: policyv1alpha1.ValidateRuleTypeCondition,
					Condition: &policyv1alpha1.ValidateCondition{
						ValueRef: &policyv1alpha1.ResourceRefer{
							From: policyv1alpha1.FromOwnerReference,
						},
					},
				},
			},
			want: &CueParams{
				ExtraParams: map[string]any{
					"otherObject": deployObj,
				},
			},
			wantErr: false,
		},
		{
			name: "http",
			args: args{
				c:         dc,
				curObject: pod,
				condition: &policyv1alpha1.ValidateRuleTemplate{
					Type: policyv1alpha1.ValidateRuleTypeCondition,
					Condition: &policyv1alpha1.ValidateCondition{
						ValueRef: &policyv1alpha1.ResourceRefer{
							From: policyv1alpha1.FromHTTP,
							Http: &policyv1alpha1.HttpDataRef{
								URL:    "http://127.0.0.1:8090/api/v1/token",
								Method: "GET",
								Params: map[string]string{
									"val": "{{metadata.name}}",
								},
							},
						},
					},
				},
			},
			want: &CueParams{
				ExtraParams: map[string]any{
					"http": map[string]any{
						"body": map[string]any{
							"token": "pod",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "k8s_d",
			args: args{
				keySuffix: "_d",
				c:         dc,
				curObject: pod,
				condition: &policyv1alpha1.ValidateRuleTemplate{
					Type: policyv1alpha1.ValidateRuleTypeCondition,
					Condition: &policyv1alpha1.ValidateCondition{
						DataRef: &policyv1alpha1.ResourceRefer{
							From: policyv1alpha1.FromK8s,
							K8s: &policyv1alpha1.ResourceSelector{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
								Namespace:  "{{metadata.namespace}}",
								Name:       "{{metadata.annotations.deploy-name}}",
							},
						},
					},
				},
			},
			want: &CueParams{
				ExtraParams: map[string]any{
					"otherObject_d": deployObj,
				},
			},
			wantErr: false,
		},
		{
			name: "owner_d",
			args: args{
				keySuffix: "_d",
				c:         dc,
				curObject: pod,
				condition: &policyv1alpha1.ValidateRuleTemplate{
					Type: policyv1alpha1.ValidateRuleTypeCondition,
					Condition: &policyv1alpha1.ValidateCondition{
						DataRef: &policyv1alpha1.ResourceRefer{
							From: policyv1alpha1.FromOwnerReference,
						},
					},
				},
			},
			want: &CueParams{
				ExtraParams: map[string]any{
					"otherObject_d": deployObj,
				},
			},
			wantErr: false,
		},
		{
			name: "http_d",
			args: args{
				keySuffix: "_d",
				c:         dc,
				curObject: pod,
				condition: &policyv1alpha1.ValidateRuleTemplate{
					Type: policyv1alpha1.ValidateRuleTypeCondition,
					Condition: &policyv1alpha1.ValidateCondition{
						DataRef: &policyv1alpha1.ResourceRefer{
							From: policyv1alpha1.FromHTTP,
							Http: &policyv1alpha1.HttpDataRef{
								URL:    "http://127.0.0.1:8090/api/v1/token",
								Method: "GET",
								Params: map[string]string{
									"val": "{{metadata.name}}",
								},
							},
						},
					},
				},
			},
			want: &CueParams{
				ExtraParams: map[string]any{
					"http_d": map[string]any{
						"body": map[string]any{
							"token": "pod",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "pab_owner",
			args: args{
				c:         dc,
				curObject: pod,
				condition: &policyv1alpha1.ValidateRuleTemplate{
					Type: policyv1alpha1.ValidateRuleTypePodAvailableBadge,
					PodAvailableBadge: &policyv1alpha1.PodAvailableBadge{
						MaxUnavailable: &intstr.IntOrString{
							Type:   intstr.String,
							StrVal: "60%",
						},
						ReplicaReference: &policyv1alpha1.ReplicaResourceRefer{
							From: policyv1alpha1.FromOwnerReference,
						},
					},
				},
			},
			want: &CueParams{
				ExtraParams: map[string]any{
					"otherObject": deployObj,
				},
			},
			wantErr: false,
		},
		{
			name: "pab_k8s",
			args: args{
				c:         dc,
				curObject: pod,
				condition: &policyv1alpha1.ValidateRuleTemplate{
					Type: policyv1alpha1.ValidateRuleTypePodAvailableBadge,
					PodAvailableBadge: &policyv1alpha1.PodAvailableBadge{
						MaxUnavailable: &intstr.IntOrString{
							Type:   intstr.String,
							StrVal: "60%",
						},
						ReplicaReference: &policyv1alpha1.ReplicaResourceRefer{
							From: policyv1alpha1.FromK8s,
							K8s: &policyv1alpha1.ResourceSelector{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
								Namespace:  "{{metadata.namespace}}",
								Name:       "{{metadata.annotations.deploy-name}}",
							},
						},
					},
				},
			},
			want: &CueParams{
				ExtraParams: map[string]any{
					"otherObject": deployObj,
				},
			},
			wantErr: false,
		},
		{
			name: "pab_http",
			args: args{
				c:         dc,
				curObject: pod,
				condition: &policyv1alpha1.ValidateRuleTemplate{
					Type: policyv1alpha1.ValidateRuleTypePodAvailableBadge,
					PodAvailableBadge: &policyv1alpha1.PodAvailableBadge{
						MaxUnavailable: &intstr.IntOrString{
							Type:   intstr.String,
							StrVal: "60%",
						},
						ReplicaReference: &policyv1alpha1.ReplicaResourceRefer{
							From: policyv1alpha1.FromHTTP,
							Http: &policyv1alpha1.HttpDataRef{
								URL:    "http://127.0.0.1:8090/api/v1/token",
								Method: "GET",
								Params: map[string]string{
									"val": "{{metadata.name}}",
								},
							},
						},
					},
				},
			},
			want: &CueParams{
				ExtraParams: map[string]any{
					"http": map[string]any{
						"body": map[string]any{
							"token": "pod",
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCueParamsViaValidatePolicy(tt.args.c, tt.args.curObject, tt.args.condition)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildCueParamsViaValidatePolicy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !equalExtraParams(got.ExtraParams, tt.want.ExtraParams, tt.args.keySuffix) {
				t.Errorf("BuildCueParamsViaOverridePolicy() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_matchRefValue(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "1",
			args: args{
				s: "abc{{xx}}",
			},
			want: "xx",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchRefValue(tt.args.s); got != tt.want {
				t.Errorf("matchRefValue() = %v, want %v", got, tt.want)
			}
		})
	}
}
