package core

import "context"

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
