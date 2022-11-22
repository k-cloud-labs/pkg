package tokenmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type TokenGenerator interface {
	// Generate fetch new token and return expire time as well.
	Generate(ctx context.Context) (token string, expireAt time.Time, err error)
	// ID for identity to where the token belongs.
	ID() string
}

type tokenGeneratorImpl struct {
	id                    string
	authUrl               string
	username              string
	password              string
	defaultExpireDuration time.Duration
}

var (
	defaultExpireDuration = time.Minute * 5 // 5min as default token expire time.
)

func NewTokenGenerator(authUrl, username, password string, defaultExpire time.Duration) (TokenGenerator, error) {
	tg := &tokenGeneratorImpl{
		authUrl:               authUrl,
		username:              username,
		password:              password,
		defaultExpireDuration: defaultExpire,
	}

	// parse url
	host, err := tg.getHost()
	if err != nil {
		return nil, err
	}

	tg.id = fmt.Sprintf("%s:%s", host, username)
	return tg, nil
}

type Token struct {
	Token    string `json:"token"`
	ExpireAt int64  `json:"expireAt,omitempty"`
}

func (tg *tokenGeneratorImpl) Generate(ctx context.Context) (token string, expireAt time.Time, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	// set timeout
	ctx2, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	req, err := http.NewRequest(http.MethodPost, tg.authUrl, nil)
	if err != nil {
		return
	}
	req = req.WithContext(ctx2)
	req.SetBasicAuth(tg.username, tg.password)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer noErr(resp.Body.Close)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		err = fmt.Errorf("request for get token failed with statuscode=%v, body=%v", resp.StatusCode, string(bodyBytes))
		return
	}

	t := new(Token)
	if err = json.Unmarshal(bodyBytes, t); err != nil {
		return
	}

	if t.ExpireAt > 0 {
		expireAt = time.Unix(t.ExpireAt, 0)
		return t.Token, expireAt, nil
	}

	// use default expire duration
	if tg.defaultExpireDuration > 0 {
		return t.Token, time.Now().Add(tg.defaultExpireDuration), nil
	}

	// if user neo set own default expire duration
	return t.Token, time.Now().Add(defaultExpireDuration), nil
}

func noErr(f func() error) {
	_ = f()
}

func (tg *tokenGeneratorImpl) ID() string {
	return tg.id
}

func (tg *tokenGeneratorImpl) getHost() (string, error) {
	u, err := url.Parse(tg.authUrl)
	if err != nil {
		return "", err
	}

	return u.Host, nil
}
