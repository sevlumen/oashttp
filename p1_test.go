package oashttp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type headerAPIKeyProvider struct{}

func (headerAPIKeyProvider) SecurityScheme() SecurityScheme {
	return SecurityScheme{Type: "apiKey", Name: "X-API-Key", In: "header"}
}

func (headerAPIKeyProvider) Authenticate(_ context.Context, r *http.Request) (*Principal, error) {
	if r.Header.Get("X-API-Key") != "secret" {
		return nil, ErrUnauthorized
	}
	return &Principal{
		Subject:     "client-1",
		Features:    map[string]struct{}{"storage": {}},
		Permissions: map[string]struct{}{"upload": {}},
	}, nil
}

func TestOperationMetadataIsAvailableToHandler(t *testing.T) {
	app := New(Config{Info: Info{Title: "Metadata", Version: "1.0.0"}})
	MapGet(app.Group(""), "/items/{id}", func(ctx context.Context, _ struct {
		ID string `path:"id"`
	}) Result[map[string]string] {
		info, ok := OperationFromContext(ctx)
		if !ok {
			t.Fatal("missing operation metadata")
		}
		if info.ID != "getItem" || info.Method != http.MethodGet || info.Route != "/items/{id}" {
			t.Fatalf("info=%#v", info)
		}
		if OperationID(ctx) != "getItem" || RoutePattern(ctx) != "/items/{id}" {
			t.Fatalf("id=%q route=%q", OperationID(ctx), RoutePattern(ctx))
		}
		return OK(map[string]string{"ok": "true"})
	}).WithOperationID("getItem").Produces(http.StatusOK)

	recorder := httptest.NewRecorder()
	app.MustBuild().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/items/123", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestScopedMiddlewareRunsAfterSecurityAndInDeclaredOrder(t *testing.T) {
	app := New(Config{
		Info: Info{Title: "Middleware", Version: "1.0.0"},
		SecurityProviders: map[string]SecurityProvider{
			"apiKey": headerAPIKeyProvider{},
		},
	})
	group := app.Group("/api")
	order := []string{}
	groupMiddleware := func(name string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name+":before")
				if OperationID(r.Context()) != "upload" {
					t.Fatalf("operation=%q", OperationID(r.Context()))
				}
				if principal, ok := PrincipalFromContext(r.Context()); !ok || principal.Subject != "client-1" {
					t.Fatalf("principal=%#v ok=%v", principal, ok)
				}
				next.ServeHTTP(w, r)
				order = append(order, name+":after")
			})
		}
	}
	if err := group.Use(groupMiddleware("group")); err != nil {
		t.Fatal(err)
	}
	child := group.Group("/v1")
	if err := child.Use(groupMiddleware("child")); err != nil {
		t.Fatal(err)
	}
	MapPost(child, "/upload", func(ctx context.Context, _ struct{}) Result[map[string]bool] {
		order = append(order, "handler")
		return OK(map[string]bool{"ok": true})
	}).WithOperationID("upload").RequireSecurity("apiKey").RequireFeatureAndPermission("storage", "upload").Use(groupMiddleware("operation")).Produces(http.StatusOK)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload", nil)
	req.Header.Set("X-API-Key", "secret")
	recorder := httptest.NewRecorder()
	app.MustBuild().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	got := strings.Join(order, ",")
	want := "group:before,child:before,operation:before,handler,operation:after,child:after,group:after"
	if got != want {
		t.Fatalf("order=%q want=%q", got, want)
	}
}

func TestGroupUseRejectsNilAndFrozenApplication(t *testing.T) {
	app := New(Config{Info: Info{Title: "Group", Version: "1.0.0"}})
	group := app.Group("/api")
	if err := group.Use(nil); err == nil {
		t.Fatal("expected nil middleware error")
	}
	if _, err := app.Build(); err != nil {
		t.Fatal(err)
	}
	if err := group.Use(func(next http.Handler) http.Handler { return next }); err != ErrFrozen {
		t.Fatalf("error=%v", err)
	}
}

func TestNamedSecurityProviderGeneratesOpenAPIScheme(t *testing.T) {
	app := New(Config{
		Info: Info{Title: "Security", Version: "1.0.0"},
		SecurityProviders: map[string]SecurityProvider{
			"clientKey": headerAPIKeyProvider{},
		},
	})
	MapGet(app.Group(""), "/protected", func(context.Context, struct{}) Result[map[string]bool] {
		return OK(map[string]bool{"ok": true})
	}).WithOperationID("protected").RequireSecurity("clientKey").Produces(http.StatusOK)
	if err := app.MapOpenAPI("/openapi.json"); err != nil {
		t.Fatal(err)
	}

	unauthorized := httptest.NewRecorder()
	app.MustBuild().ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/protected", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", unauthorized.Code, unauthorized.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("X-API-Key", "secret")
	success := httptest.NewRecorder()
	app.MustBuild().ServeHTTP(success, req)
	if success.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", success.Code, success.Body.String())
	}

	doc := httptest.NewRecorder()
	app.MustBuild().ServeHTTP(doc, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	body := doc.Body.String()
	for _, expected := range []string{`"clientKey":{"type":"apiKey","name":"X-API-Key","in":"header"}`, `"security":[{"clientKey":[]}]`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("OpenAPI missing %s: %s", expected, body)
		}
	}
}

func TestRawHandlerSharesRouterSecurityMiddlewareAndOpenAPI(t *testing.T) {
	app := New(Config{
		Info: Info{Title: "Raw", Version: "1.0.0"},
		SecurityProviders: map[string]SecurityProvider{
			"clientKey": headerAPIKeyProvider{},
		},
	})
	group := app.Group("/v1")
	seenMiddleware := false
	if err := group.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seenMiddleware = true
			if OperationID(r.Context()) != "uploadObject" {
				t.Fatalf("operation=%q", OperationID(r.Context()))
			}
			next.ServeHTTP(w, r)
		})
	}); err != nil {
		t.Fatal(err)
	}

	MapHandler(group, http.MethodPost, "/objects/{id}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "stream-body" || r.PathValue("id") != "abc" {
			t.Fatalf("body=%q id=%q", data, r.PathValue("id"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})).WithOperationID("uploadObject").WithTags("Storage").RequireSecurity("clientKey").RequireFeatureAndPermission("storage", "upload").ProducesResponse(http.StatusCreated, "Uploaded", "application/json", map[string]bool{})
	if err := app.MapOpenAPI("/openapi.json"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/objects/abc", strings.NewReader("stream-body"))
	req.Header.Set("X-API-Key", "secret")
	recorder := httptest.NewRecorder()
	app.MustBuild().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusCreated || !seenMiddleware {
		t.Fatalf("status=%d middleware=%v body=%s", recorder.Code, seenMiddleware, recorder.Body.String())
	}

	doc := httptest.NewRecorder()
	app.MustBuild().ServeHTTP(doc, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	body := doc.Body.String()
	for _, expected := range []string{`"/v1/objects/{id}"`, `"operationId":"uploadObject"`, `"clientKey"`, `"201"`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("OpenAPI missing %s: %s", expected, body)
		}
	}
}

func TestRawAndTypedDuplicateRoutesReturnBuildError(t *testing.T) {
	app := New(Config{Info: Info{Title: "Duplicates", Version: "1.0.0"}})
	group := app.Group("")
	MapHandler(group, http.MethodGet, "/items/{id}", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).WithOperationID("rawItem").Produces(http.StatusOK)
	MapGet(group, "/items/{id}", func(context.Context, struct {
		ID string `path:"id"`
	}) Result[map[string]bool] {
		return OK(map[string]bool{"ok": true})
	}).WithOperationID("typedItem").Produces(http.StatusOK)
	if _, err := app.Build(); err == nil {
		t.Fatal("expected duplicate route build error")
	}
}

func TestRawBuilderMutationAfterBuildPanicsWithErrFrozen(t *testing.T) {
	app := New(Config{Info: Info{Title: "Frozen", Version: "1.0.0"}})
	builder := MapHandler(app.Group(""), http.MethodGet, "/raw", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).WithOperationID("raw").Produces(http.StatusNoContent)
	if _, err := app.Build(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if recovered := recover(); recovered != ErrFrozen {
			t.Fatalf("panic=%v", recovered)
		}
	}()
	builder.WithSummary("changed")
}
