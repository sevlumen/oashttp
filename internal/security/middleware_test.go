package security

import (
	"context"
	"errors"
	"testing"

	"github.com/quang020102/go-osm/internal/core"
)

type auth struct{ ok bool }

func (a auth) Authenticate(context.Context, string) (*core.Principal, error) {
	if !a.ok {
		return nil, errors.New("bad")
	}
	return &core.Principal{Features: map[string]struct{}{"f": {}}, Permissions: map[string]struct{}{"p": {}}}, nil
}

func TestAuthenticationAndAuthorization(t *testing.T) {
	_, failure := AuthenticateAndAuthorize(context.Background(), "Bearer token", "f", "p", auth{true}, nil)
	if failure != nil {
		t.Fatalf("failure=%#v", failure)
	}
	_, failure = AuthenticateAndAuthorize(context.Background(), "", "f", "p", auth{true}, nil)
	if failure == nil || failure.Status != 401 || failure.Challenge != "Bearer" {
		t.Fatalf("failure=%#v", failure)
	}
}

func TestBearerSchemeIsCaseInsensitive(t *testing.T) {
	_, failure := AuthenticateAndAuthorize(context.Background(), "bearer token", "f", "p", auth{true}, nil)
	if failure != nil {
		t.Fatalf("failure=%#v", failure)
	}
}

func TestAuthenticationFailuresUseRFC6750Challenges(t *testing.T) {
	cases := []struct {
		name      string
		header    string
		auth      auth
		status    int
		challenge string
	}{
		{name: "missing", status: 401, challenge: "Bearer", auth: auth{true}},
		{name: "malformed", header: "Basic token", status: 401, challenge: `Bearer error="invalid_request"`, auth: auth{true}},
		{name: "invalid", header: "Bearer token", status: 401, challenge: `Bearer error="invalid_token"`, auth: auth{false}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, failure := AuthenticateAndAuthorize(context.Background(), tc.header, "f", "p", tc.auth, nil)
			if failure == nil || failure.Status != tc.status || failure.Challenge != tc.challenge {
				t.Fatalf("failure=%#v", failure)
			}
		})
	}
}

func TestAuthorizationFailureUsesInsufficientScope(t *testing.T) {
	principalAuth := authenticatorFunc(func(context.Context, string) (*core.Principal, error) {
		return &core.Principal{Features: map[string]struct{}{}, Permissions: map[string]struct{}{}}, nil
	})
	_, failure := AuthenticateAndAuthorize(context.Background(), "Bearer token", "f", "p", principalAuth, nil)
	if failure == nil || failure.Status != 403 || failure.Challenge != `Bearer error="insufficient_scope"` {
		t.Fatalf("failure=%#v", failure)
	}
}

type authenticatorFunc func(context.Context, string) (*core.Principal, error)

func (f authenticatorFunc) Authenticate(ctx context.Context, token string) (*core.Principal, error) {
	return f(ctx, token)
}
