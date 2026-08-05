package oashttp

import (
	"context"

	"github.com/oashttp/oashttp/internal/core"
	internalsecurity "github.com/oashttp/oashttp/internal/security"
)

type Principal = core.Principal
type Authenticator = core.Authenticator
type Authorizer = core.Authorizer

func PrincipalFromContext(ctx context.Context) (*Principal, bool) {
	return internalsecurity.PrincipalFromContext(ctx)
}
