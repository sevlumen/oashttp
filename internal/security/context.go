package security

import (
	"context"

	"github.com/sevlumen/oashttp/v2/internal/core"
)

func WithPrincipal(ctx context.Context, principal *core.Principal) context.Context {
	return core.WithPrincipal(ctx, principal)
}

func PrincipalFromContext(ctx context.Context) (*core.Principal, bool) {
	return core.PrincipalFromContext(ctx)
}
