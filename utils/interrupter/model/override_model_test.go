package model

import (
	"testing"
)

func Test_handlePath(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "normal",
			args: args{path: "/spec/template/spec/containers/0/tolerations/1/key"},
			want: "spec.template.spec.containers[0].tolerations[1].key",
		},
		{
			name: "no-hande",
			args: args{path: "a.b.c.d[0]"},
			want: "a.b.c.d[0]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := handlePath(tt.args.path); got != tt.want {
				t.Errorf("handlePath() = %v, want %v", got, tt.want)
			}
		})
	}
}
