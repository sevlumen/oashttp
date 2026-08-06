package security

import (
	"context"
	"github.com/sevlumen/oashttp/v2/internal/core"
)

type principalKey struct{}

func WithPrincipal(ctx context.Context, p *core.Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}
func PrincipalFromContext(ctx context.Context) (*core.Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(*core.Principal)
	return p, ok && p != nil
}
