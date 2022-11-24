package fake

import (
	"k8s.io/klog/v2"

	"github.com/k-cloud-labs/pkg/utils/tokenmanager"
)

type FakeTokenManager struct {
}

func (f *FakeTokenManager) AddToken(generator tokenmanager.TokenGenerator, callback tokenmanager.IdentifiedCallback) {
	klog.V(4).InfoS("new token added", "id", generator.ID(), "callbackID", callback.ID())
}

func (f *FakeTokenManager) RemoveToken(tg tokenmanager.TokenGenerator, ic tokenmanager.IdentifiedCallback) {
}

func (f *FakeTokenManager) Stop() {
}

var (
	_ tokenmanager.TokenManager = &FakeTokenManager{}
)

func NewFakeTokenGenerator() *FakeTokenManager {
	return &FakeTokenManager{}
}
