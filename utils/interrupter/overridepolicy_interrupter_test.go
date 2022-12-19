package interrupter

import (
	"reflect"
	"testing"
	"time"

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
