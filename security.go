package oashttp

import (
	"context"

	"github.com/sevlumen/oashttp/v2/internal/core"
	internalsecurity "github.com/sevlumen/oashttp/v2/internal/security"
)

type Principal = core.Principal
type Authenticator = core.Authenticator
type Authorizer = core.Authorizer
type SecurityScheme = core.SecurityScheme
type SecurityProvider = core.SecurityProvider

var ErrUnauthorized = internalsecurity.ErrUnauthorized
var ErrForbidden = internalsecurity.ErrForbidden

func PrincipalFromContext(ctx context.Context) (*Principal, bool) {
	return internalsecurity.PrincipalFromContext(ctx)
}
