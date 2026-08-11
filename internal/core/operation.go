package core

import (
	"context"
	"sync"
)

type operationContextKey struct{}

type operationCarrier struct {
	mu   sync.RWMutex
	info OperationInfo
	ok   bool
}

// OperationInfo describes the operation selected for a request.
type OperationInfo struct {
	ID     string
	Method string
	Route  string
}

// WithOperationCarrier installs a request-scoped mutable carrier. It is used
// by the outer application pipeline so middleware that wraps routing can read
// operation metadata after the routed handler returns.
func WithOperationCarrier(ctx context.Context) context.Context {
	if carrier, ok := ctx.Value(operationContextKey{}).(*operationCarrier); ok && carrier != nil {
		return ctx
	}
	return context.WithValue(ctx, operationContextKey{}, &operationCarrier{})
}

// WithOperationInfo records the selected operation. If a carrier already
// exists, it is updated in place so outer middleware retains visibility.
func WithOperationInfo(ctx context.Context, info OperationInfo) context.Context {
	if carrier, ok := ctx.Value(operationContextKey{}).(*operationCarrier); ok && carrier != nil {
		carrier.mu.Lock()
		carrier.info = info
		carrier.ok = true
		carrier.mu.Unlock()
		return ctx
	}
	carrier := &operationCarrier{info: info, ok: true}
	return context.WithValue(ctx, operationContextKey{}, carrier)
}

func OperationFromContext(ctx context.Context) (OperationInfo, bool) {
	carrier, ok := ctx.Value(operationContextKey{}).(*operationCarrier)
	if !ok || carrier == nil {
		return OperationInfo{}, false
	}
	carrier.mu.RLock()
	defer carrier.mu.RUnlock()
	return carrier.info, carrier.ok
}
