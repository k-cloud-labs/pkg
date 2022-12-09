package tokenmanager

import (
	"context"
	"errors"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
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
	RemoveToken(tg TokenGenerator, ic IdentifiedCallback)
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

	klog.V(4).InfoS("addToken", "token.ID", generator.ID(), "callbackAll.ID", ic.ID())

	info, ok := t.tokenMap[generator.ID()]
	if !ok {
		info = &tokenMaintainer{
			name:        generator.ID(),
			generator:   generator,
			callbackMap: sync.Map{},
			stopChan:    make(chan struct{}, 1),
			valueLock:   new(sync.RWMutex),
		}
	}

	info.updateCallbacks(ic)
	t.tokenMap[generator.ID()] = info
	if !ok {
		go info.daemon()
	} else {
		// callback immediately
		go info.callback(ic)
	}
}

func (t *tokenManagerImpl) RemoveToken(tg TokenGenerator, ic IdentifiedCallback) {
	t.mu.Lock()
	defer t.mu.Unlock()
	klog.V(4).InfoS("removeToken", "token.ID", tg.ID(), "callbackAll.ID", ic.ID())

	info, ok := t.tokenMap[tg.ID()]
	if !ok {
		return
	}

	if lastOne := info.removeCallback(ic); !lastOne {
		return
	}

	klog.V(4).InfoS("stop token", "token.ID", tg.ID(), "callbackAll.ID", ic.ID())
	delete(t.tokenMap, tg.ID())
	go info.stop() // block channel
}

func (t *tokenManagerImpl) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	for id, maintainer := range t.tokenMap {
		klog.V(4).InfoS("stopping token maintain", "tokenID", id)
		maintainer.stop()
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
	name      string
	generator TokenGenerator
	stopChan  chan struct{}

	valueLock *sync.RWMutex
	token     string
	fetchedAt time.Time
	expireAt  time.Time

	callbackMap sync.Map
}

func (t *tokenMaintainer) updateCallbacks(ic IdentifiedCallback) {
	if ic == nil {
		return
	}

	t.callbackMap.Store(ic.ID(), ic)
}

func (t *tokenMaintainer) callback(ic IdentifiedCallback) error {
	token, expireAt, _ := t.getValues()
	if retryErr := retry(func() error {
		return ic.Callback(token, expireAt)
	}, 3, time.Millisecond*100); retryErr != nil {
		klog.ErrorS(retryErr, "retry callback failed", "id", ic.ID())
		return retryErr
	}

	return nil
}

func (t *tokenMaintainer) callbackAll() error {

	eg, _ := errgroup.WithContext(context.Background())
	t.callbackMap.Range(func(_, value any) bool {
		ic := value.(IdentifiedCallback)
		eg.Go(func() error {
			return t.callback(ic)
		})

		return true
	})

	return eg.Wait()
}

// return true if callbackAll map is empty
func (t *tokenMaintainer) removeCallback(ic IdentifiedCallback) bool {
	t.callbackMap.Delete(ic.ID())
	var empty = true
	t.callbackMap.Range(func(key, value any) bool {
		empty = false
		return false
	})
	return empty
}

func (t *tokenMaintainer) stop() {
	t.stopChan <- struct{}{}
}

func (t *tokenMaintainer) refreshToken() error {
	// fetch token first
	token, expireAt, err := t.generator.Generate(context.Background())
	if err != nil {
		return err
	}

	_, _, fetchedAt := t.getValues()
	// token must be at least valid for one minute
	if expireAt.Before(fetchedAt) || expireAt.Sub(fetchedAt).Seconds() < 60 {
		return errors.New("token valid duration too short or expired already, please extend the valid time")
	}

	t.setValues(token, expireAt)
	return nil
}

func (t *tokenMaintainer) setValues(token string, expireAt time.Time) {
	t.valueLock.Lock()
	defer t.valueLock.Unlock()

	t.token = token
	t.expireAt = expireAt
	t.fetchedAt = time.Now()
}

func (t *tokenMaintainer) getValues() (token string, expireAt, fetchAt time.Time) {
	t.valueLock.RLock()
	defer t.valueLock.RUnlock()

	return t.token, t.expireAt, t.fetchedAt
}

func (t *tokenMaintainer) refreshAndCallback() error {
	if err := t.refreshToken(); err != nil {
		klog.ErrorS(err, "refresh token got error", "id", t.generator.ID())
		return err
	}

	return t.callbackAll()
}

func (t *tokenMaintainer) daemon() {
	// attempt to refresh token when it started.
	for {
		select {
		case <-t.stopChan:
			klog.V(4).InfoS("token maintainer stop", "name", t.name)
			return
		default:
		}
		err := t.refreshAndCallback()
		if err == nil {
			break
		}
		klog.ErrorS(err, "refresh token got error", "id", t.generator.ID())
		time.Sleep(time.Millisecond * 100)
	}

	_, expireAt, fetchedAt := t.getValues()
	var (
		expireSeconds = expireAt.Sub(fetchedAt).Seconds()
		duration      = expireSeconds / 10
		ticker        = time.NewTicker(time.Duration(duration) * time.Second)
		heartBeat     = time.NewTicker(time.Second)
		refreshFailed = true
	)

	for {
		select {
		case <-t.stopChan:
			klog.V(4).InfoS("token maintainer stop", "name", t.name)
			return
		case <-ticker.C:
			err := t.refreshAndCallback()
			if err != nil {
				refreshFailed = true
				continue
			}
		case <-heartBeat.C:
			// refreshFailed default value is true, so it will refresh token at first here
			if refreshFailed {
				// will retry 3 times inside
				if err := t.refreshAndCallback(); err != nil {
					klog.ErrorS(err, "refresh token got error", "id", t.generator.ID())
					continue
				}
				// reset
				refreshFailed = false
			}
		}
	}
}

func retry(f func() error, retryTimes int, interval time.Duration) error {
	var (
		count int
		err   error
	)
retry:
	if count >= retryTimes {
		klog.ErrorS(err, "retry failed", "retryTimes", count)
		return err
	}

	if err = f(); err == nil {
		return nil
	}

	count++
	time.Sleep(interval * time.Duration(count))
	goto retry
}
