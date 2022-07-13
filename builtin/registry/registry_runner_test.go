package registry

import (
	"testing"

	"cuelang.org/go/cue"
	"github.com/bmizerany/assert"
)

func TestContext(t *testing.T) {
	var r cue.Runtime

	lpV := `test: "just a test"`
	inst, err := r.Compile("lp", lpV)
	if err != nil {
		t.Error(err)
		return
	}
	ctx := Meta{Obj: inst.Value()}
	val := ctx.Lookup("test")
	assert.Equal(t, true, val.Exists())

	intV := `iTest: 64`
	iInst, err := r.Compile("int", intV)
	if err != nil {
		t.Error(err)
		return
	}
	iCtx := Meta{Obj: iInst.Value()}
	iVal := iCtx.Int64("iTest")
	assert.Equal(t, int64(64), iVal)
}

func TestRunner(t *testing.T) {
	key := "mock"
	RegisterRunner(key, newMockRunner)

	task := LookupRunner(key)
	if task == nil {
		t.Errorf("there is no task %s", key)
	}
	runner, err := task(cue.Value{})
	if err != nil {
		t.Errorf("fail to get runner, %v", err)
	}
	rs, err := runner.Run(&Meta{Obj: cue.Value{}})
	assert.Equal(t, nil, err)
	assert.Equal(t, "mock", rs)
}

func newMockRunner(v cue.Value) (Runner, error) {
	return &MockRunner{name: "mock"}, nil
}

type MockRunner struct {
	name string
}

func (r *MockRunner) Run(meta *Meta) (res interface{}, err error) {
	return r.name, nil
}
