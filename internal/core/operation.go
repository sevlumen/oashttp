package core

import (
	"context"
	"sync"
)

type operationContextKey struct{}

type operationCarrier struct {
	mu          sync.RWMutex
	info        OperationInfo
	operationOK bool
	principal   *Principal
}

// OperationInfo describes the operation selected for a request.
type OperationInfo struct {
	ID     string
	Method string
	Route  string
}

// WithOperationCarrier installs the request-scoped mutable state used by the
// outer application pipeline. Routing and authentication update this state so
// middleware that wraps the router can inspect it after next returns.
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
		carrier.operationOK = true
		carrier.mu.Unlock()
		return ctx
	}
	carrier := &operationCarrier{info: info, operationOK: true}
	return context.WithValue(ctx, operationContextKey{}, carrier)
}

func OperationFromContext(ctx context.Context) (OperationInfo, bool) {
	carrier, ok := ctx.Value(operationContextKey{}).(*operationCarrier)
	if !ok || carrier == nil {
		return OperationInfo{}, false
	}
	carrier.mu.RLock()
	defer carrier.mu.RUnlock()
	return carrier.info, carrier.operationOK
}

// WithPrincipal records the authenticated principal in the same request state
// used for operation metadata. This preserves visibility for both inner and
// outer middleware without exposing mutable state publicly.
func WithPrincipal(ctx context.Context, principal *Principal) context.Context {
	if carrier, ok := ctx.Value(operationContextKey{}).(*operationCarrier); ok && carrier != nil {
		carrier.mu.Lock()
		carrier.principal = principal
		carrier.mu.Unlock()
		return ctx
	}
	carrier := &operationCarrier{principal: principal}
	return context.WithValue(ctx, operationContextKey{}, carrier)
}

func PrincipalFromContext(ctx context.Context) (*Principal, bool) {
	carrier, ok := ctx.Value(operationContextKey{}).(*operationCarrier)
	if !ok || carrier == nil {
		return nil, false
	}
	carrier.mu.RLock()
	defer carrier.mu.RUnlock()
	return carrier.principal, carrier.principal != nil
}
