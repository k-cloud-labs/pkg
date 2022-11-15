package templatemanager

import (
	"encoding/json"
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/format"
	"cuelang.org/go/cue/parser"
	"cuelang.org/go/cue/token"
)

// CueManager defines methods of manage cue string, it provides various methods tomake sure the cue is valid.
type CueManager interface {
	// Format - format the cue string, it gets same result of running `cue fmt xxx.cue`
	Format(src []byte, opts ...format.Option) (result []byte, err error)
	// Validate - check the cue whether valid or not, it returns error if it's invalid
	// Same as running `cue vet xxx.cue -c`
	Validate(src []byte, opts ...cue.Option) error
	// DryRun - calculate/render cue with given data.
	// Same as running `cue eval xxx.cue`
	DryRun(src []byte, data any, outputField string) (result []byte, err error)
	// Exec -  execute cue with given data
	Exec(src []byte, data any, outputField string, output any) error
}

type cueManagerImpl struct {
	debug bool
}

// NewCueManager init a new CueManager
func NewCueManager() CueManager {
	return &cueManagerImpl{
		debug: true,
	}
}

func (c *cueManagerImpl) Format(src []byte, opts ...format.Option) (result []byte, err error) {
	formatOptions := opts
	if len(opts) == 0 {
		formatOptions = []format.Option{
			format.UseSpaces(4),
			format.TabIndent(false),
		}
	}

	res, err := format.Source(src, formatOptions...)
	if err != nil {
		return nil, err
	}

	// make sure formatted output is syntactically correct
	if _, err := parser.ParseFile("", res, parser.AllErrors); err != nil {
		return nil, errors.Append(err.(errors.Error),
			errors.Newf(token.NoPos, "re-parse failed: %s", res))
	}

	return res, nil
}

func (c *cueManagerImpl) Validate(src []byte, opts ...cue.Option) error {
	validateOptions := opts
	if len(opts) == 0 {
		validateOptions = []cue.Option{
			cue.Attributes(true),
			cue.Definitions(true),
			cue.Hidden(true),
		}
	}
	bi := build.NewContext().NewInstance("", nil)
	// add template
	fs, err := parser.ParseFile("-", src, parser.ParseComments)
	if err != nil {
		return err
	}

	if err = bi.AddSyntax(fs); err != nil {
		return err
	}

	value := cuecontext.New().BuildInstance(bi)
	err = value.Validate(validateOptions...)
	if err != nil {
		return err
	}

	return err
}

func (c *cueManagerImpl) DryRun(src []byte, data any, outputField string) (result []byte, err error) {
	bi := build.NewContext().NewInstance("", nil)
	// add template
	fs, err := parser.ParseFile("-", src, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	if err = bi.AddSyntax(fs); err != nil {
		return nil, err
	}

	if data != nil {
		bt, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}

		paramFile := fmt.Sprintf("data: %s", string(bt))
		fs, err = parser.ParseFile("parameter", paramFile)
		if err != nil {
			return nil, err
		}

		if err = bi.AddSyntax(fs); err != nil {
			return nil, err
		}
	}

	value := cuecontext.New().BuildInstance(bi)
	value = value.Eval()
	if value.Err() != nil {
		return nil, value.Err()
	}
	// 2. generate result
	value = value.LookupPath(cue.ParsePath(outputField))
	if !value.Exists() {
		return nil, value.Err()
	}

	return value.MarshalJSON()
}

func (c *cueManagerImpl) Exec(src []byte, data any, outputField string, output any) error {
	bi := build.NewContext().NewInstance("", nil)
	// add template
	fs, err := parser.ParseFile("-", src, parser.ParseComments)
	if err != nil {
		return err
	}

	if err = bi.AddSyntax(fs); err != nil {
		return err
	}

	if data != nil {
		bt, err := json.Marshal(data)
		if err != nil {
			return err
		}

		paramFile := fmt.Sprintf("data: %s", string(bt))
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
	value.Expr()

	if err = value.Validate(); err != nil {
		return err
	}

	// 2. generate result
	value = value.LookupPath(cue.ParsePath(outputField))
	if !value.Exists() {
		return value.Err()
	}

	if err := value.Decode(output); err != nil {
		return err
	}

	return nil
}
