package core

import (
	"context"
	"net/http"
)

type Principal struct {
	Subject     string
	Features    map[string]struct{}
	Permissions map[string]struct{}
	Claims      map[string]any
}

type Authenticator interface {
	Authenticate(context.Context, string) (*Principal, error)
}

type Authorizer interface {
	Authorize(context.Context, *Principal, string, string) error
}

// SecurityScheme describes the OpenAPI security scheme exposed by a provider.
// The current named-provider model covers HTTP and API-key schemes.
type SecurityScheme struct {
	Type         string
	Scheme       string
	BearerFormat string
	Name         string
	In           string
}

// SecurityProvider authenticates directly from an HTTP request. This keeps
// credential extraction scheme-specific instead of coupling it to Authorization.
type SecurityProvider interface {
	SecurityScheme() SecurityScheme
	Authenticate(context.Context, *http.Request) (*Principal, error)
}
