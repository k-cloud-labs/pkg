package tokenmanager

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

type test_callback struct {
	id       string
	callback func(token string, expireAt time.Time) error
}

func (t *test_callback) ID() string {
	return t.id
}

func (t *test_callback) Callback(token string, expireAt time.Time) error {
	return t.callback(token, expireAt)
}

type test_tokenGeneratorImpl struct {
	id                    string
	defaultExpireDuration time.Duration
}

func (tg *test_tokenGeneratorImpl) Generate(_ context.Context) (token string, expireAt time.Time, err error) {
	// use default expire duration
	if tg.defaultExpireDuration > 0 {
		return "token", time.Now().Add(tg.defaultExpireDuration), nil
	}

	// if user neo set own default expire duration
	return "token", time.Now().Add(defaultExpireDuration), nil
}

func (tg *test_tokenGeneratorImpl) ID() string {
	return tg.id
}

func Test_tokenMaintainer_callback(t1 *testing.T) {
	type fields struct {
		generator TokenGenerator
		cb        IdentifiedCallback
	}
	tests := []struct {
		name   string
		fields fields
	}{
		{
			name: "1",
			fields: fields{
				generator: &test_tokenGeneratorImpl{
					id:                    "t1",
					defaultExpireDuration: time.Minute * 2,
				},
				cb: &test_callback{
					id: "t1",
					callback: func(token string, expireAt time.Time) error {
						if token == "token" {
							return nil
						}

						if expireAt.Before(time.Now()) {
							return fmt.Errorf("token expired")
						}

						return fmt.Errorf("token is not correct(%v)", token)
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &tokenMaintainer{
				generator: tt.fields.generator,
				callbacks: make(map[string]IdentifiedCallback),
				stopChan:  make(chan struct{}),
				mu:        new(sync.RWMutex),
			}
			t.updateCallbacks(tt.fields.cb)
			if err := t.refreshToken(); err != nil {
				t1.Error(err)
				return
			}

			t.callback()
		})
	}
}

func Test_tokenMaintainer_refreshAndCallback(t1 *testing.T) {
	type fields struct {
		generator TokenGenerator
		cb        IdentifiedCallback
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "normal",
			fields: fields{
				generator: &test_tokenGeneratorImpl{
					id:                    "t1",
					defaultExpireDuration: time.Minute * 2,
				},
				cb: &test_callback{
					id: "t1",
					callback: func(token string, expireAt time.Time) error {
						if token == "token" {
							return nil
						}

						if expireAt.Before(time.Now()) {
							return fmt.Errorf("token expired")
						}

						return fmt.Errorf("token is not correct(%v)", token)
					},
				},
			},
			wantErr: false,
		},
		{
			name: "error",
			fields: fields{
				generator: &test_tokenGeneratorImpl{
					id:                    "t1",
					defaultExpireDuration: time.Second * 30,
				},
				cb: &test_callback{
					id: "t1",
					callback: func(token string, expireAt time.Time) error {
						if token == "token" {
							return nil
						}

						if expireAt.Before(time.Now()) {
							return fmt.Errorf("token expired")
						}

						return fmt.Errorf("token is not correct(%v)", token)
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &tokenMaintainer{
				generator: tt.fields.generator,
				callbacks: make(map[string]IdentifiedCallback),
				stopChan:  make(chan struct{}),
				mu:        new(sync.RWMutex),
			}
			if err := t.refreshAndCallback(); (err != nil) != tt.wantErr {
				t1.Errorf("refreshAndCallback() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_tokenMaintainer_removeCallback(t1 *testing.T) {
	type fields struct {
		generator TokenGenerator
		cb        IdentifiedCallback
	}
	type args struct {
		ic IdentifiedCallback
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "normal",
			fields: fields{
				generator: &test_tokenGeneratorImpl{
					id:                    "t1",
					defaultExpireDuration: time.Minute * 2,
				},
				cb: &test_callback{
					id: "t1",
					callback: func(token string, expireAt time.Time) error {
						if token == "token" {
							return nil
						}

						if expireAt.Before(time.Now()) {
							return fmt.Errorf("token expired")
						}

						return fmt.Errorf("token is not correct(%v)", token)
					},
				},
			},
			args: args{ic: &test_callback{id: "t1"}},
			want: true,
		},
		{
			name: "false",
			fields: fields{
				generator: &test_tokenGeneratorImpl{
					id:                    "t1",
					defaultExpireDuration: time.Minute * 2,
				},
			},
			args: args{ic: &test_callback{id: "t1"}},
			want: true,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &tokenMaintainer{
				generator: tt.fields.generator,
				callbacks: make(map[string]IdentifiedCallback),
				stopChan:  make(chan struct{}),
				mu:        new(sync.RWMutex),
			}

			t.updateCallbacks(tt.fields.cb)
			if got := t.removeCallback(tt.args.ic); got != tt.want {
				t1.Errorf("removeCallback() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_tokenManagerImpl_AddToken(t1 *testing.T) {
	type fields struct {
		tg TokenManager
	}
	type args struct {
		generator TokenGenerator
		ic        IdentifiedCallback
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "1",
			fields: fields{
				tg: NewTokenManager(),
			},
			args: args{
				generator: &test_tokenGeneratorImpl{
					id:                    "t1",
					defaultExpireDuration: time.Minute * 2,
				},
				ic: &test_callback{
					id: "t1",
					callback: func(token string, expireAt time.Time) error {
						if token == "token" {
							return nil
						}

						if expireAt.Before(time.Now()) {
							return fmt.Errorf("token expired")
						}

						return fmt.Errorf("token is not correct(%v)", token)
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			tt.fields.tg.AddToken(tt.args.generator, tt.args.ic)
		})
	}
}

func Test_tokenManagerImpl_RemoveToken(t1 *testing.T) {
	type fields struct {
		tm TokenManager
	}
	type args struct {
		tg TokenGenerator
		ic IdentifiedCallback
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "1",
			fields: fields{
				tm: NewTokenManager(),
			},
			args: args{
				tg: &test_tokenGeneratorImpl{
					id:                    "t1",
					defaultExpireDuration: time.Minute * 2,
				},
				ic: &test_callback{
					id: "t1",
					callback: func(token string, expireAt time.Time) error {
						if token == "token" {
							return nil
						}

						if expireAt.Before(time.Now()) {
							return fmt.Errorf("token expired")
						}

						return fmt.Errorf("token is not correct(%v)", token)
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			tt.fields.tm.AddToken(tt.args.tg, tt.args.ic)
			tt.fields.tm.RemoveToken(tt.args.tg, tt.args.ic)
		})
	}
}

func Test_tokenManagerImpl_Stop(t1 *testing.T) {
	type fields struct {
		tm TokenManager
	}
	tests := []struct {
		name   string
		fields fields
	}{
		{
			name: "1",
			fields: fields{
				tm: NewTokenManager(),
			},
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			tt.fields.tm.Stop()
		})
	}
}

func Test_retry(t *testing.T) {
	type args struct {
		f          func() error
		retryTimes int
		interval   time.Duration
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "normal",
			args: args{
				f: func() error {
					return nil
				},
				retryTimes: 3,
				interval:   time.Millisecond * 10,
			},
			wantErr: false,
		},
		{
			name: "error",
			args: args{
				f: func() error {
					return fmt.Errorf("must error")
				},
				retryTimes: 3,
				interval:   time.Millisecond * 10,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := retry(tt.args.f, tt.args.retryTimes, tt.args.interval); (err != nil) != tt.wantErr {
				t.Errorf("retry() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
