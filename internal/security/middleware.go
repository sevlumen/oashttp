package security

import (
	"context"
	"errors"
	"strings"

	"github.com/quang020102/go-osm/internal/core"
)

var ErrUnauthorized = errors.New("unauthorized")
var ErrForbidden = errors.New("forbidden")

func AuthenticateAndAuthorize(ctx context.Context, header, feature, permission string, authenticator core.Authenticator, authorizer core.Authorizer) (context.Context, int, string, string) {
	if authenticator == nil {
		return ctx, 500, "SECURITY_NOT_CONFIGURED", "Authenticator is not configured"
	}
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return ctx, 401, "UNAUTHORIZED", "A bearer token is required"
	}
	token := parts[1]
	principal, err := authenticator.Authenticate(ctx, token)
	if err != nil || principal == nil {
		return ctx, 401, "UNAUTHORIZED", "The bearer token is invalid"
	}
	if authorizer != nil {
		if err := authorizer.Authorize(ctx, principal, feature, permission); err != nil {
			return ctx, 403, "FORBIDDEN", "The principal does not have the required permission"
		}
	} else {
		if _, ok := principal.Features[feature]; !ok {
			return ctx, 403, "FORBIDDEN", "The principal does not have the required feature"
		}
		if _, ok := principal.Permissions[permission]; !ok {
			return ctx, 403, "FORBIDDEN", "The principal does not have the required permission"
		}
	}
	return WithPrincipal(ctx, principal), 0, "", ""
}
