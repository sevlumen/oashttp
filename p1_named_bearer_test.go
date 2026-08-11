package oashttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type requestBearerProvider struct{}

func (requestBearerProvider) SecurityScheme() SecurityScheme {
	return SecurityScheme{Type: "http", Scheme: "bearer", BearerFormat: "JWT"}
}

func (requestBearerProvider) Authenticate(_ context.Context, r *http.Request) (*Principal, error) {
	if r.Header.Get("Authorization") != "Bearer good" {
		return nil, ErrUnauthorized
	}
	return &Principal{Subject: "client"}, nil
}

func TestNamedBearerProviderEmitsHTTPChallenges(t *testing.T) {
	app := New(Config{
		Info: Info{Title: "Named bearer", Version: "1.0.0"},
		SecurityProviders: map[string]SecurityProvider{
			"clientJWT": requestBearerProvider{},
		},
	})
	MapGet(app.Group(""), "/protected", func(context.Context, struct{}) Result[map[string]bool] {
		return OK(map[string]bool{"ok": true})
	}).WithOperationID("namedBearerProtected").RequireSecurity("clientJWT").Produces(http.StatusOK)

	unauthorized := httptest.NewRecorder()
	app.MustBuild().ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/protected", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", unauthorized.Code, unauthorized.Body.String())
	}
	if challenge := unauthorized.Header().Get("WWW-Authenticate"); challenge != "Bearer" {
		t.Fatalf("challenge=%q", challenge)
	}
}
