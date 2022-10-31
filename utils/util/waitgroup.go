package util

import (
	"sync"
)

type WaitGroup struct {
	wg *sync.WaitGroup
	ch chan struct{}
}

func NewWaitGroup(size int) *WaitGroup {
	return &WaitGroup{
		wg: new(sync.WaitGroup),
		ch: make(chan struct{}, size),
	}
}

func (w *WaitGroup) Add(f func()) {
	w.ch <- struct{}{}
	w.wg.Add(1)
	go func() {
		defer w.Done()
		f()
	}()
}

func (w *WaitGroup) Done() {
	<-w.ch
	w.wg.Done()
}

func (w *WaitGroup) Wait() {
	w.wg.Wait()
}
