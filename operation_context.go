package oashttp

import (
	"context"

	"github.com/sevlumen/oashttp/v2/internal/core"
)

type OperationInfo = core.OperationInfo

// OperationFromContext returns metadata for the selected operation.
func OperationFromContext(ctx context.Context) (OperationInfo, bool) {
	return core.OperationFromContext(ctx)
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
