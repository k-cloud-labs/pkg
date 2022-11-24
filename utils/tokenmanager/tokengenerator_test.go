package tokenmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newMockHttpServer mock the http server
func newMockHttpServer() *httptest.Server {
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "POST" {
			fmt.Printf("Expected 'GET' or 'POST' request, got '%s'", r.Method)
		}
		if r.URL.EscapedPath() != "/api/v1/auth" {
			fmt.Printf("Expected request to '/api/v1/token', got '%s'", r.URL.EscapedPath())
		}
		_ = r.ParseForm()
		token := r.Header.Get("Authorization")
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

func TestNewTokenGenerator(t *testing.T) {
	type args struct {
		authUrl       string
		username      string
		password      string
		defaultExpire time.Duration
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "normal",
			args: args{
				authUrl:       "https://auth.shopee.io/auth",
				username:      "kinitiras",
				password:      "****",
				defaultExpire: time.Hour,
			},
			wantErr: false,
		},
		{
			name: "error",
			args: args{
				authUrl:       "authUrl",
				username:      "kinitiras",
				password:      "****",
				defaultExpire: time.Hour,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewTokenGenerator(tt.args.authUrl, tt.args.username, tt.args.password, tt.args.defaultExpire)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewTokenGenerator() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func Test_tokenGeneratorImpl_Generate(t *testing.T) {
	hs := newMockHttpServer()
	defer hs.Close()

	type fields struct {
		authUrl               string
		username              string
		password              string
		defaultExpireDuration time.Duration
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name         string
		fields       fields
		args         args
		wantToken    string
		wantExpireAt time.Time
		wantErr      bool
	}{
		{
			name: "1",
			fields: fields{
				authUrl:               "http://127.0.0.1:8090/api/v1/auth",
				username:              "kinitiras",
				password:              "****",
				defaultExpireDuration: time.Hour,
			},
			args: args{
				ctx: context.Background(),
			},
			wantToken:    "Basic a2luaXRpcmFzOioqKio=",
			wantExpireAt: time.Now().Add(time.Hour),
			wantErr:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tg, err := NewTokenGenerator(tt.fields.authUrl, tt.fields.username, tt.fields.password, tt.fields.defaultExpireDuration)
			if err != nil {
				t.Errorf("NewTokenGenerator() error = %v", err)
				return
			}
			gotToken, gotExpireAt, err := tg.Generate(tt.args.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Generate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotToken != tt.wantToken {
				t.Errorf("Generate() gotToken = %v, want %v", gotToken, tt.wantToken)
			}
			if gotExpireAt.Second() != tt.wantExpireAt.Second() {
				t.Errorf("Generate() gotExpireAt = %v, want %v", gotExpireAt, tt.wantExpireAt)
			}
		})
	}
}

func Test_tokenGeneratorImpl_ID(t *testing.T) {
	type fields struct {
		id                    string
		authUrl               string
		username              string
		password              string
		defaultExpireDuration time.Duration
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "1",
			fields: fields{
				authUrl:               "http://127.0.0.1:8090/api/v1/auth",
				username:              "kinitiras",
				password:              "****",
				defaultExpireDuration: time.Hour,
			},
			want: "127.0.0.1:8090:kinitiras",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tg, err := NewTokenGenerator(tt.fields.authUrl, tt.fields.username, tt.fields.password, tt.fields.defaultExpireDuration)
			if err != nil {
				t.Errorf("NewTokenGenerator() error = %v", err)
				return
			}
			if got := tg.ID(); got != tt.want {
				t.Errorf("ID() = %v, want %v", got, tt.want)
			}
		})
	}
}
