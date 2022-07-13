package cue

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k-cloud-labs/pkg/test/helper"
	"github.com/k-cloud-labs/pkg/utils"
)

func TestCueDoAndReturn(t *testing.T) {
	s := newMockHttpServer()
	defer s.Close()

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
	reason: "hello cue"
	valid: object.metadata.name == "ut-cue-success-with-parameter"
}
`,
			parameters: []Parameter{
				{
					Name:   utils.ObjectParameterName,
					Object: helper.NewDeployment(metav1.NamespaceDefault, "ut-cue-success-with-parameter"),
				},
			},
			outputName: "validate",
			output: &struct {
				Reason string `json:"reason"`
				Valid  bool   `json:"valid"`
			}{},
			wantedOutput: &struct {
				Reason string `json:"reason"`
				Valid  bool   `json:"valid"`
			}{
				Reason: "hello cue",
				Valid:  true,
			},
			wantedErr: nil,
		},
		{
			name: "cue-success-without-parameter",
			cue: `
validate:{
	reason: "hello cue"
	valid: true
}
`,
			parameters: []Parameter{
				{
					Name:   utils.ObjectParameterName,
					Object: nil,
				},
			},
			outputName: "validate",
			output: &struct {
				Reason string `json:"reason"`
				Valid  bool   `json:"valid"`
			}{},
			wantedOutput: &struct {
				Reason string `json:"reason"`
				Valid  bool   `json:"valid"`
			}{
				Reason: "hello cue",
				Valid:  true,
			},
			wantedErr: nil,
		},
		{
			name: "cue-failed-output-type",
			cue: `
object: _ @tag(object)

validate:{
	reason: "hello cue"
	valid: object.metadata.name == "ut-cue-failed-output-type"
}
`,
			parameters: []Parameter{
				{
					Name:   utils.ObjectParameterName,
					Object: helper.NewDeployment(metav1.NamespaceDefault, "ut-cue-failed-output-type"),
				},
			},
			outputName: "validate",
			output: struct {
				Reason string `json:"reason"`
				Valid  bool   `json:"valid"`
			}{},
			wantedOutput: struct {
				Reason string `json:"reason"`
				Valid  bool   `json:"valid"`
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
	reason: "hello cue"
	valid: object.metadata.name == "ut-cue-failed-output-nil"
}
`,
			parameters: []Parameter{
				{
					Name:   utils.ObjectParameterName,
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
					Name:   utils.ObjectParameterName,
					Object: helper.NewDeployment(metav1.NamespaceDefault, "ut-cue-failed-without-output"),
				},
			},
			outputName: "validate",
			output: &struct {
				Reason string `json:"reason"`
				Valid  bool   `json:"valid"`
			}{},
			wantedOutput: &struct {
				Reason string `json:"reason"`
				Valid  bool   `json:"valid"`
			}{
				Valid: false,
			},
			wantedErr: OutputNotFoundErr,
		},
		{
			name: "cue-success-with-processing-http",
			cue: `
object: _ @tag(object)

processing: {
  http: {
    method: *"GET" | string
    url: "http://127.0.0.1:8090/api/v1/token?val=test-token"
    request: {
      body ?: bytes
      header: {
        "Accept-Language": "en,nl"
      }
      trailer: {
        "Accept-Language": "en,nl"
        User: "foo"
      }
    }
    output: {
      token?: string
    }
  }
}

validate:{
	reason: "hello cue"
	valid: object.metadata.name == "ut-cue-success-with-parameter" && processing.output.token == "test-token"
}
`,
			parameters: []Parameter{
				{
					Name:   utils.ObjectParameterName,
					Object: helper.NewDeployment(metav1.NamespaceDefault, "ut-cue-success-with-parameter"),
				},
			},
			outputName: "validate",
			output: &struct {
				Reason string `json:"reason"`
				Valid  bool   `json:"valid"`
			}{},
			wantedOutput: &struct {
				Reason string `json:"reason"`
				Valid  bool   `json:"valid"`
			}{
				Reason: "hello cue",
				Valid:  true,
			},
			wantedErr: nil,
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

// newMockHttpServer mock the http server
func newMockHttpServer() *httptest.Server {
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			fmt.Printf("Expected 'GET' request, got '%s'", r.Method)
		}
		if r.URL.EscapedPath() != "/api/v1/token" {
			fmt.Printf("Expected request to '/api/v1/token', got '%s'", r.URL.EscapedPath())
		}
		_ = r.ParseForm()
		token := r.Form.Get("val")
		tokenBytes, _ := json.Marshal(map[string]interface{}{"token": token})

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(tokenBytes)
	}))
	l, _ := net.Listen("tcp", "127.0.0.1:8090")
	ts.Listener.Close()
	ts.Listener = l
	ts.Start()
	return ts
}
