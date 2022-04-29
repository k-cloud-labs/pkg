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
)

var (
	ParameterNotFoundErr = errors.New("parameter not found in cue")
	OutputNilErr         = errors.New("output must not be nil")
	OutputNotSettableErr = errors.New("output must be settable")
)

// CueDoAndReturn will execute cue code and set execution result to output.
// output must not be nil and must be settable.
func CueDoAndReturn(template string, parameterName string, parameter interface{}, outputName string, output interface{}) error {
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

	// add parameter
	if !isNil(parameter) {
		bt, err := json.Marshal(parameter)
		if err != nil {
			return err
		}

		paramFile := fmt.Sprintf("%s: %s", parameterName, string(bt))
		fs, err = parser.ParseFile("parameter", paramFile)
		if err != nil {
			return err
		}

		if err = bi.AddSyntax(fs); err != nil {
			return err
		}
	}

	// execute cue
	value := cuecontext.New().BuildInstance(bi)
	if err = value.Validate(); err != nil {
		return err
	}

	result := value.LookupPath(cue.ParsePath(outputName))
	if !result.Exists() {
		return ParameterNotFoundErr
	}

	if err := result.Decode(output); err != nil {
		return err
	}

	return nil
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
