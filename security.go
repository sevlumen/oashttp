package oashttp

import (
	"context"

	"github.com/quang020102/go-osm/internal/core"
	internalsecurity "github.com/quang020102/go-osm/internal/security"
)

type Principal = core.Principal
type Authenticator = core.Authenticator
type Authorizer = core.Authorizer

func PrincipalFromContext(ctx context.Context) (*Principal, bool) {
	return internalsecurity.PrincipalFromContext(ctx)
}
