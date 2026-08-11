package oashttp

import "context"

type operationContextKey struct{}

// OperationInfo describes the operation selected by oashttp for the request.
type OperationInfo struct {
	ID     string
	Method string
	Route  string
}

func withOperationInfo(ctx context.Context, info OperationInfo) context.Context {
	return context.WithValue(ctx, operationContextKey{}, info)
}

// OperationFromContext returns metadata for the selected operation.
func OperationFromContext(ctx context.Context) (OperationInfo, bool) {
	info, ok := ctx.Value(operationContextKey{}).(OperationInfo)
	return info, ok
}

// OperationID returns the selected OpenAPI operation ID, or an empty string.
func OperationID(ctx context.Context) string {
	info, _ := OperationFromContext(ctx)
	return info.ID
}

// RoutePattern returns the normalized route pattern, or an empty string.
func RoutePattern(ctx context.Context) string {
	info, _ := OperationFromContext(ctx)
	return info.Route
}
