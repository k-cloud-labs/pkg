package tokenmanager

import (
	"context"
	"errors"
	"sync"
	"time"

	"k8s.io/klog/v2"
)

type IdentifiedCallback interface {
	ID() string
	Callback(token string, expireAt time.Time) error
}

// TokenManager provides cache and maintain token ability.
type TokenManager interface {
	// AddToken add new token to manager
	AddToken(TokenGenerator, IdentifiedCallback)
	// RemoveToken stops maintaining process of given token and remove it from cache.
	RemoveToken(tg TokenGenerator, ic IdentifiedCallback) error
	// Stop stops all token maintaining and clean the cache, don't use this manager after call Stop.
	Stop()
}

type tokenManagerImpl struct {
	tokenMap map[string]*tokenMaintainer
	mu       *sync.RWMutex
}

func (t *tokenManagerImpl) AddToken(generator TokenGenerator, ic IdentifiedCallback) {
	t.mu.Lock()
	defer t.mu.Unlock()

	info, ok := t.tokenMap[generator.ID()]
	if !ok {
		info = &tokenMaintainer{
			generator: generator,
			callbacks: make(map[string]IdentifiedCallback),
			stopChan:  make(chan struct{}),
			mu:        new(sync.RWMutex),
		}
	}

	info.updateCallbacks(ic)
	t.tokenMap[generator.ID()] = info
	if !ok {
		go info.daemon()
	}
}

func (t *tokenManagerImpl) RemoveToken(generator TokenGenerator, ic IdentifiedCallback) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	info, ok := t.tokenMap[generator.ID()]
	if !ok {
		return nil
	}

	if lastOne := info.removeCallback(ic); !lastOne {
		return nil
	}

	// no more callback, stop maintain token
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
	defer cancel()
	return info.stop(ctx.Done())
}

func (t *tokenManagerImpl) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for id, maintainer := range t.tokenMap {
		if err := maintainer.stop(ctx.Done()); err != nil {
			klog.ErrorS(err, "stop token maintainer failed", "id", id)
		}
	}

	t.tokenMap = nil
	t.mu = nil
}

// NewTokenManager return an implement of TokenManager.
func NewTokenManager() TokenManager {
	return &tokenManagerImpl{
		tokenMap: make(map[string]*tokenMaintainer),
		mu:       new(sync.RWMutex),
	}
}

type tokenMaintainer struct {
	generator TokenGenerator
	token     string
	fetchedAt time.Time
	expireAt  time.Time
	stopChan  chan struct{}

	mu        *sync.RWMutex
	callbacks map[string]IdentifiedCallback
}

func (t *tokenMaintainer) updateCallbacks(ic IdentifiedCallback) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.callbacks[ic.ID()] = ic
}

func (t *tokenMaintainer) callback() {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for id, cb := range t.callbacks {
		if err := cb.Callback(t.token, t.expireAt); err != nil {
			go retry(func() error {
				return cb.Callback(t.token, t.expireAt)
			}, 3, time.Millisecond*100)

			klog.ErrorS(err, "call refresh token callback error", "id", id)
		}
	}
}

// return true if callback map is empty
func (t *tokenMaintainer) removeCallback(ic IdentifiedCallback) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.callbacks, ic.ID())
	return len(t.callbacks) == 0
}

func (t *tokenMaintainer) stop(wait <-chan struct{}) error {
	select {
	case t.stopChan <- struct{}{}:
		return nil
	case <-wait:
		return errors.New("stop token maintain timeout")
	}
}

func (t *tokenMaintainer) refreshToken() error {
	// fetch token first
	token, expireAt, err := t.generator.Generate(context.Background())
	if err != nil {
		return err
	}

	t.fetchedAt = time.Now()
	// token must be at least valid for one minute
	if expireAt.Before(t.fetchedAt) || expireAt.Sub(t.fetchedAt).Minutes() < 1 {
		return errors.New("token valid duration too short or expired already, please extend the valid time")
	}

	t.token = token
	t.expireAt = expireAt
	return nil
}

func (t *tokenMaintainer) refreshAndCallback() error {
	if err := t.refreshToken(); err != nil {
		klog.ErrorS(err, "refresh token got error", "lastToken", t.token, "id", t.generator.ID())
		return err
	}

	t.callback()
	return nil
}

func (t *tokenMaintainer) daemon() {
	var (
		expireSeconds = t.expireAt.Sub(t.fetchedAt).Seconds()
		duration      = expireSeconds / 10
		ticker        = time.NewTicker(time.Duration(duration) * time.Second)
		heartBeat     = time.NewTicker(time.Second)
		// attempt to refresh token when it started.
		refreshFailed = true
	)

	for {
		select {
		case <-t.stopChan:
			return
		case <-ticker.C:
			err := t.refreshAndCallback()
			if err != nil {
				refreshFailed = true
				continue
			}
		default:
		}

		// refreshFailed default value is true, so it will refresh token at first here
		if refreshFailed {
			if err := t.refreshAndCallback(); err != nil {
				klog.ErrorS(err, "refresh token got error", "lastToken", t.token, "id", t.generator.ID())
				time.Sleep(time.Millisecond * 10) // wait for 10ms before next call
				continue
			}
			// reset
			refreshFailed = false
		}

		// heart beat
		// wait for heart beat if there are nothing happen.
		<-heartBeat.C
	}
}

func retry(f func() error, retryTimes int, interval time.Duration) {
	var (
		count int
	)
retry:
	if count >= retryTimes {
		return
	}

	if err := f(); err == nil {
		return
	}

	count++
	time.Sleep(interval * time.Duration(count))
	goto retry
}
