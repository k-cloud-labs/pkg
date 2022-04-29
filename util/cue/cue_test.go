package cue

import (
	"reflect"
	"testing"

	"github.com/k-cloud-labs/pkg/test/helper"
)

func TestCueDoAndReturn(t *testing.T) {
	tests := []struct {
		name          string
		cue           string
		parameterName string
		parameter     interface{}
		outputName    string
		output        interface{}
		wantedErr     error
		wantedOutput  interface{}
	}{
		{
			name: "cue-success-with-parameter",
			cue: `
object: _ @tag(object)

validate:[{
	msg: "hello cue"
	valid: object.metadata.name == "ut-cue-success-with-parameter"
}]
`,
			parameterName: "object",
			parameter:     helper.NewDeployment("ut-cue-success-with-parameter", "default"),
			outputName:    "validate",
			output: &[]struct {
				Msg   string `json:"msg"`
				Valid bool   `json:"valid"`
			}{},
			wantedOutput: &[]struct {
				Msg   string `json:"msg"`
				Valid bool   `json:"valid"`
			}{
				{
					Msg:   "hello cue",
					Valid: true,
				},
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
			parameterName: "object",
			parameter:     nil,
			outputName:    "validate",
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
			parameterName: "object",
			parameter:     helper.NewDeployment("ut-cue-failed-output-type", "default"),
			outputName:    "validate",
			output: struct {
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
			wantedErr: OutputNotSettableErr,
		},
		{
			name: "cue-failed-output-type",
			cue: `
object: _ @tag(object)

validate:{
	msg: "hello cue"
	valid: object.metadata.name == "ut-cue-failed-output-nil"
}
`,
			parameterName: "object",
			parameter:     helper.NewDeployment("ut-cue-failed-output-nil", "default"),
			outputName:    "validate",
			output:        nil,
			wantedOutput: &struct {
				Msg   string `json:"msg"`
				Valid bool   `json:"valid"`
			}{
				Msg:   "hello cue",
				Valid: true,
			},
			wantedErr: OutputNilErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CueDoAndReturn(tt.cue, tt.parameterName, tt.parameter, tt.outputName, tt.output); !reflect.DeepEqual(got, tt.wantedErr) {
				t.Errorf("CueDoAndReturn() = %v, output = %v, want: %v, %v", got, tt.output, tt.wantedErr, tt.wantedOutput)
			}
		})
	}
}
