package security

import (
	"context"
	"errors"
	"github.com/quang020102/go-osm/internal/core"
	"testing"
)

type auth struct{ ok bool }

func (a auth) Authenticate(context.Context, string) (*core.Principal, error) {
	if !a.ok {
		return nil, errors.New("bad")
	}
	return &core.Principal{Features: map[string]struct{}{"f": {}}, Permissions: map[string]struct{}{"p": {}}}, nil
}
func TestAuthenticationAndAuthorization(t *testing.T) {
	_, status, _, _ := AuthenticateAndAuthorize(context.Background(), "Bearer token", "f", "p", auth{true}, nil)
	if status != 0 {
		t.Fatalf("status=%d", status)
	}
	_, status, _, _ = AuthenticateAndAuthorize(context.Background(), "", "f", "p", auth{true}, nil)
	if status != 401 {
		t.Fatalf("status=%d", status)
	}
}

func TestBearerSchemeIsCaseInsensitive(t *testing.T) {
	_, status, _, _ := AuthenticateAndAuthorize(context.Background(), "bearer token", "f", "p", auth{true}, nil)
	if status != 0 {
		t.Fatalf("status=%d", status)
	}
}
