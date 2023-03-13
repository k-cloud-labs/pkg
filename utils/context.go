package utils

import (
	"context"

	utiltrace "k8s.io/utils/trace"
)

// ContextForChannel derives a child context from a parent channel.
//
// The derived context's Done channel is closed when the returned cancel function
// is called or when the parent channel is closed, whichever happens first.
//
// Note the caller must *always* call the CancelFunc, otherwise resources may be leaked.
func ContextForChannel(parentCh <-chan struct{}) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		select {
		case <-parentCh:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

type traceCtxKey struct{}

// ContextWithTrace return a new context with trace as value.
func ContextWithTrace(ctx context.Context, trace *utiltrace.Trace) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	return context.WithValue(ctx, traceCtxKey{}, trace)
}

func TraceFromContext(ctx context.Context) *utiltrace.Trace {
	v, ok := ctx.Value(traceCtxKey{}).(*utiltrace.Trace)
	if ok {
		return v
	}

	return nil
}
