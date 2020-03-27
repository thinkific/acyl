package context

import (
	"context"
)

const (
	cfContextKey = "cancel_func"
)

type CancelFuncContextKey string

// NewCancelFuncContext returns a context with a CancelFunc embedded as a value
// This allows downstream handlers to cancel themselves and child contexts all the way up the callstack
func NewCancelFuncContext(ctx context.Context, cancelFunc context.CancelFunc) context.Context {
	return context.WithValue(ctx, CancelFuncContextKey(cfContextKey), cancelFunc)
}

// GetCancelFunc returns the CancelFunc if present or a stub function that does nothing
func GetCancelFunc(ctx context.Context) context.CancelFunc {
	cf, ok := ctx.Value(CancelFuncContextKey(cfContextKey)).(context.CancelFunc)
	if cf == nil || !ok {
		return func() {}
	}
	return cf
}
