package fake

import (
	"context"
	"time"

	"github.com/k-cloud-labs/pkg/utils/tokenmanager"
)

type FakeTokenGeneratorImpl struct {
	id                    string
	defaultExpireDuration time.Duration
}

var (
	_ tokenmanager.TokenGenerator = &FakeTokenGeneratorImpl{}
)

var (
	defaultExpireDuration = time.Minute * 5 // 5min as default token expire time.
)

func NewTokenGenerator(authUrl, username, password string, defaultExpire time.Duration) *FakeTokenGeneratorImpl {
	return &FakeTokenGeneratorImpl{
		id:                    authUrl + username + password,
		defaultExpireDuration: defaultExpire,
	}
}

func (tg *FakeTokenGeneratorImpl) Generate(_ context.Context) (token string, expireAt time.Time, err error) {
	// use default expire duration
	if tg.defaultExpireDuration > 0 {
		return "token", time.Now().Add(tg.defaultExpireDuration), nil
	}

	// if user neo set own default expire duration
	return "token", time.Now().Add(defaultExpireDuration), nil
}

func (tg *FakeTokenGeneratorImpl) Equal(t1 tokenmanager.TokenGenerator) bool {
	return tg.id == t1.ID()
}

func (tg *FakeTokenGeneratorImpl) ID() string {
	return tg.id
}
