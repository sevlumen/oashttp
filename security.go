package oashttp

import (
	"context"
	"strings"

	"github.com/sevlumen/oashttp/v2/internal/core"
	internalsecurity "github.com/sevlumen/oashttp/v2/internal/security"
)

type Principal = core.Principal
type Authenticator = core.Authenticator
type Authorizer = core.Authorizer
type SecurityScheme = core.SecurityScheme
type SecurityProvider = core.SecurityProvider

var ErrUnauthorized = internalsecurity.ErrUnauthorized

func PrincipalFromContext(ctx context.Context) (*Principal, bool) {
	return internalsecurity.PrincipalFromContext(ctx)
}

func normalizeSecurityName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		panic("oashttp: security name cannot be empty")
	}
	if name == "bearerAuth" {
		panic("oashttp: security name bearerAuth is reserved for Config.Authenticator")
	}
	if !isOpenAPIComponentName(name) {
		panic("oashttp: security name must match ^[a-zA-Z0-9.\\-_]+$")
	}
	return name
}

func isOpenAPIComponentName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}
