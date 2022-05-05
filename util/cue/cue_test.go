package cue

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k-cloud-labs/pkg/test/helper"
	"github.com/k-cloud-labs/pkg/util"
)

func TestCueDoAndReturn(t *testing.T) {
	tests := []struct {
		name         string
		cue          string
		parameters   []Parameter
		outputName   string
		output       interface{}
		wantedErr    error
		wantedOutput interface{}
	}{
		{
			name: "cue-success-with-parameter",
			cue: `
object: _ @tag(object)

validate:{
	msg: "hello cue"
	valid: object.metadata.name == "ut-cue-success-with-parameter"
}
`,
			parameters: []Parameter{
				{
					Name:   util.ObjectParameterName,
					Object: helper.NewDeployment(metav1.NamespaceDefault, "ut-cue-success-with-parameter"),
				},
			},
			outputName: "validate",
			output: &struct {
				Msg   string `json:"msg"`
				Valid bool   `json:"valid"`
			}{},
			wantedOutput: &struct {
				Msg   string `json:"msg"`
				Valid bool   `json:"valid"`
			}{
				Msg:   "hello cue",
				Valid: true,
			},
			wantedErr: nil,
		},
		{
			name: "cue-success-without-parameter",
			cue: `
validate:{
	msg: "hello cue"
	valid: true
}
`,
			parameters: []Parameter{
				{
					Name:   util.ObjectParameterName,
					Object: nil,
				},
			},
			outputName: "validate",
			output: &struct {
				Msg   string `json:"msg"`
				Valid bool   `json:"valid"`
			}{},
			wantedOutput: &struct {
				Msg   string `json:"msg"`
				Valid bool   `json:"valid"`
			}{
				Msg:   "hello cue",
				Valid: true,
			},
			wantedErr: nil,
		},
		{
			name: "cue-failed-output-type",
			cue: `
object: _ @tag(object)

validate:{
	msg: "hello cue"
	valid: object.metadata.name == "ut-cue-failed-output-type"
}
`,
			parameters: []Parameter{
				{
					Name:   util.ObjectParameterName,
					Object: helper.NewDeployment(metav1.NamespaceDefault, "ut-cue-failed-output-type"),
				},
			},
			outputName: "validate",
			output: struct {
				Msg   string `json:"msg"`
				Valid bool   `json:"valid"`
			}{},
			wantedOutput: struct {
				Msg   string `json:"msg"`
				Valid bool   `json:"valid"`
			}{
				Valid: false,
			},
			wantedErr: OutputNotSettableErr,
		},
		{
			name: "cue-failed-output-nil",
			cue: `
object: _ @tag(object)

validate:{
	msg: "hello cue"
	valid: object.metadata.name == "ut-cue-failed-output-nil"
}
`,
			parameters: []Parameter{
				{
					Name:   util.ObjectParameterName,
					Object: helper.NewDeployment(metav1.NamespaceDefault, "ut-cue-failed-output-nil"),
				},
			},
			outputName:   "validate",
			output:       nil,
			wantedOutput: nil,
			wantedErr:    OutputNilErr,
		},
		{
			name: "cue-failed-without-output",
			cue: `
object: _ @tag(object)
`,
			parameters: []Parameter{
				{
					Name:   util.ObjectParameterName,
					Object: helper.NewDeployment(metav1.NamespaceDefault, "ut-cue-failed-without-output"),
				},
			},
			outputName: "validate",
			output: &struct {
				Msg   string `json:"msg"`
				Valid bool   `json:"valid"`
			}{},
			wantedOutput: &struct {
				Msg   string `json:"msg"`
				Valid bool   `json:"valid"`
			}{
				Valid: false,
			},
			wantedErr: OutputNotFoundErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CueDoAndReturn(tt.cue, tt.parameters, tt.outputName, tt.output); !reflect.DeepEqual(got, tt.wantedErr) ||
				!reflect.DeepEqual(tt.output, tt.wantedOutput) {
				t.Errorf("CueDoAndReturn() = %v, output = %v, want: %v, %v", got, tt.output, tt.wantedErr, tt.wantedOutput)
			}
		})
	}
}
