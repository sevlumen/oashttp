package security

import (
	"context"
	"errors"
	"strings"

	"github.com/quang020102/go-osm/internal/core"
)

var ErrUnauthorized = errors.New("unauthorized")
var ErrForbidden = errors.New("forbidden")

type Failure struct {
	Status    int
	Code      string
	Detail    string
	Challenge string
}

func AuthenticateAndAuthorize(ctx context.Context, header, feature, permission string, authenticator core.Authenticator, authorizer core.Authorizer) (context.Context, *Failure) {
	if authenticator == nil {
		return ctx, &Failure{Status: 500, Code: "SECURITY_NOT_CONFIGURED", Detail: "Authenticator is not configured"}
	}
	parts := strings.Fields(header)
	if len(parts) == 0 {
		return ctx, &Failure{Status: 401, Code: "UNAUTHORIZED", Detail: "A bearer token is required", Challenge: "Bearer"}
	}
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return ctx, &Failure{Status: 401, Code: "UNAUTHORIZED", Detail: "The Authorization header is malformed", Challenge: `Bearer error="invalid_request"`}
	}
	principal, err := authenticator.Authenticate(ctx, parts[1])
	if err != nil || principal == nil {
		return ctx, &Failure{Status: 401, Code: "UNAUTHORIZED", Detail: "The bearer token is invalid", Challenge: `Bearer error="invalid_token"`}
	}
	if authorizer != nil {
		if err := authorizer.Authorize(ctx, principal, feature, permission); err != nil {
			return ctx, &Failure{Status: 403, Code: "FORBIDDEN", Detail: "The principal does not have the required permission", Challenge: `Bearer error="insufficient_scope"`}
		}
	} else {
		if _, ok := principal.Features[feature]; !ok {
			return ctx, &Failure{Status: 403, Code: "FORBIDDEN", Detail: "The principal does not have the required feature", Challenge: `Bearer error="insufficient_scope"`}
		}
		if _, ok := principal.Permissions[permission]; !ok {
			return ctx, &Failure{Status: 403, Code: "FORBIDDEN", Detail: "The principal does not have the required permission", Challenge: `Bearer error="insufficient_scope"`}
		}
	}
	return WithPrincipal(ctx, principal), nil
}
