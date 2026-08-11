package oashttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAppMiddlewareCanReadPrincipalAfterNext(t *testing.T) {
	app := New(Config{
		Info: Info{Title: "Principal observability", Version: "1.0.0"},
		SecurityProviders: map[string]SecurityProvider{
			"clientKey": headerAPIKeyProvider{},
		},
	})
	seenSubject := ""
	if err := app.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
			if principal, ok := PrincipalFromContext(r.Context()); ok {
				seenSubject = principal.Subject
			}
		})
	}); err != nil {
		t.Fatal(err)
	}
	MapGet(app.Group(""), "/protected", func(context.Context, struct{}) Result[map[string]bool] {
		return OK(map[string]bool{"ok": true})
	}).WithOperationID("observedProtected").RequireSecurity("clientKey").Produces(http.StatusOK)

	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("X-API-Key", "secret")
	recorder := httptest.NewRecorder()
	app.MustBuild().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if seenSubject != "client-1" {
		t.Fatalf("subject=%q", seenSubject)
	}
}
