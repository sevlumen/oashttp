package oashttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAppMiddlewareCanReadOperationMetadataAfterNext(t *testing.T) {
	app := New(Config{Info: Info{Title: "Observability", Version: "1.0.0"}})
	var seen OperationInfo
	var ok bool
	if err := app.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, exists := OperationFromContext(r.Context()); exists {
				t.Fatal("operation metadata must not be populated before routing")
			}
			next.ServeHTTP(w, r)
			seen, ok = OperationFromContext(r.Context())
		})
	}); err != nil {
		t.Fatal(err)
	}
	MapGet(app.Group(""), "/items/{id}", func(context.Context, struct {
		ID string `path:"id"`
	}) Result[map[string]bool] {
		return OK(map[string]bool{"ok": true})
	}).WithOperationID("getObservedItem").Produces(http.StatusOK)

	recorder := httptest.NewRecorder()
	app.MustBuild().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/items/123", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if !ok || seen.ID != "getObservedItem" || seen.Method != http.MethodGet || seen.Route != "/items/{id}" {
		t.Fatalf("operation=%#v ok=%v", seen, ok)
	}
}

func TestRawHandlerEnforcesRouteConstraints(t *testing.T) {
	app := New(Config{Info: Info{Title: "Raw constraints", Version: "1.0.0"}})
	called := false
	MapHandler(app.Group(""), http.MethodGet, "/objects/{id:uuid}", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})).WithOperationID("getRawObject").Produces(http.StatusNoContent)

	recorder := httptest.NewRecorder()
	app.MustBuild().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/objects/not-a-uuid", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if called {
		t.Fatal("raw handler must not run when a route constraint fails")
	}
}
