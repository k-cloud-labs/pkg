package cue

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/parser"
	"k8s.io/klog/v2"

	_ "github.com/k-cloud-labs/pkg/builtin/http"
	"github.com/k-cloud-labs/pkg/builtin/registry"
)

var (
	OutputNotFoundErr    = errors.New("output not found in cue")
	OutputNilErr         = errors.New("output must not be nil")
	OutputNotSettableErr = errors.New("output must be settable")
)

type Parameter struct {
	Name   string
	Object interface{}
}

// CueDoAndReturn will execute cue code and set execution result to output.
// output must not be nil and must be settable.
func CueDoAndReturn(template string, parameters []Parameter, outputName string, output interface{}) error {
	// output check
	if isNil(output) {
		return OutputNilErr
	}

	if !isSettable(output) {
		return OutputNotSettableErr
	}

	bi := build.NewContext().NewInstance("", nil)

	// add template
	fs, err := parser.ParseFile("-", template, parser.ParseComments)
	if err != nil {
		return err
	}

	if err = bi.AddSyntax(fs); err != nil {
		return err
	}

	// add parameters
	for _, parameter := range parameters {
		if !isNil(parameter) {
			bt, err := json.Marshal(parameter.Object)
			if err != nil {
				return err
			}

			paramFile := fmt.Sprintf("%s: %s", parameter.Name, string(bt))
			fs, err = parser.ParseFile("parameter", paramFile)
			if err != nil {
				return err
			}

			if err = bi.AddSyntax(fs); err != nil {
				return err
			}
		}
	}

	// execute cue
	value := cuecontext.New().BuildInstance(bi)
	if err = value.Validate(); err != nil {
		return err
	}

	// 1. execute http task
	v, err := process(&value)
	if err != nil {
		return err
	}
	value = *v

	// 2. generate result
	result := value.LookupPath(cue.ParsePath(outputName))
	if !result.Exists() {
		return OutputNotFoundErr
	}

	if err := result.Decode(output); err != nil {
		return err
	}

	return nil
}

func process(v *cue.Value) (*cue.Value, error) {
	taskVal := v.LookupPath(cue.ParsePath("processing.http"))
	if !taskVal.Exists() {
		klog.InfoS("there is no http in processing")
		return v, nil
	}
	resp, err := exec(taskVal)
	if err != nil {
		return nil, fmt.Errorf("fail to exec http task, %w", err)
	}

	value := v.FillPath(cue.ParsePath("processing.output"), resp)

	return &value, nil
}

func exec(v cue.Value) (map[string]interface{}, error) {
	runner, err := getRunnerByKey("http", v)
	if err != nil {
		return nil, err
	}

	got, err := runner.Run(&registry.Meta{Obj: v})
	if err != nil {
		return nil, err
	}
	gotMap, ok := got.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("fail to convert got to map")
	}
	body, ok := gotMap["body"].(string)
	if !ok {
		return nil, fmt.Errorf("fail to convert body to string")
	}
	resp := make(map[string]interface{})
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func getRunnerByKey(key string, v cue.Value) (registry.Runner, error) {
	task := registry.LookupRunner(key)
	if task == nil {
		return nil, errors.New("there is no http task in task registry")
	}

	runner, err := task(v)
	if err != nil {
		return nil, err
	}

	return runner, nil
}

func isNil(i interface{}) bool {
	if i == nil {
		return true
	}

	vi := reflect.ValueOf(i)
	if vi.Kind() == reflect.Ptr {
		return vi.IsNil()
	}

	return false
}

func isSettable(i interface{}) bool {
	return reflect.ValueOf(i).Kind() == reflect.Ptr
}
