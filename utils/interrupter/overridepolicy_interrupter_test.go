package interrupter

import (
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/utils/tokenmanager"
)

func Test_compareCallbackMap(t *testing.T) {
	type args struct {
		cur map[string]*tokenCallbackImpl
		old map[string]*tokenCallbackImpl
	}
	tests := []struct {
		name       string
		args       args
		wantUpdate map[string]*tokenCallbackImpl
		wantRemove map[string]*tokenCallbackImpl
	}{
		{
			name: "1",
			args: args{
				cur: map[string]*tokenCallbackImpl{
					"1": {
						id:        "1",
						generator: tokenmanager.NewTokenGenerator("1", "1", "1", time.Hour),
					},
					"2": {
						id:        "2",
						generator: tokenmanager.NewTokenGenerator("2", "1", "1", time.Hour),
					},
				},
				old: map[string]*tokenCallbackImpl{
					"3": {
						id:        "3",
						generator: tokenmanager.NewTokenGenerator("3", "1", "1", time.Hour),
					},
				},
			},
			wantUpdate: map[string]*tokenCallbackImpl{
				"1": {
					id:        "1",
					generator: tokenmanager.NewTokenGenerator("1", "1", "1", time.Hour),
				},
				"2": {
					id:        "2",
					generator: tokenmanager.NewTokenGenerator("2", "1", "1", time.Hour),
				},
			},
			wantRemove: map[string]*tokenCallbackImpl{
				"3": {
					id:        "3",
					generator: tokenmanager.NewTokenGenerator("3", "1", "1", time.Hour),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotUpdate, gotRemove := compareCallbackMap(tt.args.cur, tt.args.old)
			if !reflect.DeepEqual(gotUpdate, tt.wantUpdate) {
				t.Errorf("compareCallbackMap() gotUpdate = %v, want %v", gotUpdate, tt.wantUpdate)
			}
			if !reflect.DeepEqual(gotRemove, tt.wantRemove) {
				t.Errorf("compareCallbackMap() gotRemove = %v, want %v", gotRemove, tt.wantRemove)
			}
		})
	}
}

func Test_validateOverrideRuleOrigin(t *testing.T) {
	testCases := []struct {
		name       string
		overriders []policyv1alpha1.OverrideRuleOrigin
		expected   bool
	}{
		{
			name: "Valid Case",
			overriders: []policyv1alpha1.OverrideRuleOrigin{
				{Type: policyv1alpha1.OverrideRuleOriginResourceRequirements, ContainerCount: 1},
				{Type: policyv1alpha1.OverrideRuleOriginResourceOversell, ContainerCount: 1},
			},
			expected: true,
		},
		{
			name: "Invalid Case",
			overriders: []policyv1alpha1.OverrideRuleOrigin{
				{Type: policyv1alpha1.OverrideRuleOriginResourceRequirements, ContainerCount: 1},
				{Type: policyv1alpha1.OverrideRuleOriginResourceOversell, ContainerCount: 2},
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := validateOverrideRuleOrigin(tc.overriders)
			if result != tc.expected {
				t.Errorf("Expected %v, but got %v", tc.expected, result)
			}
		})
	}
}

func Test_validateTolerations(t *testing.T) {
	testCases := []struct {
		name        string
		tolerations []corev1.Toleration
		error       bool
	}{
		{
			name: "Valid Case",
			tolerations: []corev1.Toleration{
				{Key: "key1", Operator: corev1.TolerationOpEqual, Value: "value1"},
				{Key: "", Operator: corev1.TolerationOpExists},
			},
			error: false,
		},
		{
			name: "Invalid Case",
			tolerations: []corev1.Toleration{
				{Key: "key1", Operator: corev1.TolerationOpEqual, Value: "!@#"},
				{Key: "key2", Operator: corev1.TolerationOpExists, Value: "value2"},
			},
			error: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := validateTolerations(tc.tolerations)
			errCount := len(result)
			if (errCount > 0) != tc.error {
				t.Errorf("Expected error is %v, but got %v", tc.error, result)
			}
		})
	}
}
