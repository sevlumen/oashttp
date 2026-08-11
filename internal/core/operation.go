package core

import "context"

type operationContextKey struct{}

// OperationInfo describes the operation selected for a request.
type OperationInfo struct {
	ID     string
	Method string
	Route  string
}

func WithOperationInfo(ctx context.Context, info OperationInfo) context.Context {
	return context.WithValue(ctx, operationContextKey{}, info)
}

func OperationFromContext(ctx context.Context) (OperationInfo, bool) {
	info, ok := ctx.Value(operationContextKey{}).(OperationInfo)
	return info, ok
}
