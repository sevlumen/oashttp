package oashttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type getUserInput struct {
	ID UUID `path:"id"`
}
type userDTO struct {
	ID UUID `json:"id"`
}

func TestMapGetRegistersTypedOperation(t *testing.T) {
	app := New(Config{Info: Info{Title: "Users API", Version: "1.0.0"}})
	group := app.Group("/api/v1")
	MapGet(group, "/users/{id:uuid}", func(_ context.Context, input getUserInput) Result[userDTO] { return OK(userDTO{ID: input.ID}) }).WithOperationID("getUser").WithTags("Core / Users").WithSummary("Get user").Produces(http.StatusOK).ProducesProblem(http.StatusBadRequest)
	handler, err := app.Build()
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/users/550e8400-e29b-41d4-a716-446655440000", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

type updateBody struct {
	Name string `json:"name" validate:"required"`
}

type updateInput struct {
	ID   UUID       `path:"id"`
	Body updateBody `body:"json"`
}

type testAuthenticator struct {
	principal *Principal
	err       error
}

func (a testAuthenticator) Authenticate(context.Context, string) (*Principal, error) {
	return a.principal, a.err
}

func TestProtectedOperationEmitsBearerChallenges(t *testing.T) {
	app := New(Config{
		Info: Info{Title: "Users", Version: "1.0.0"},
		Authenticator: testAuthenticator{principal: &Principal{
			Features:    map[string]struct{}{"users": {}},
			Permissions: map[string]struct{}{"read": {}},
		}},
	})
	MapGet(app.Group(""), "/users/{id:uuid}", func(context.Context, getUserInput) Result[userDTO] {
		return OK(userDTO{})
	}).WithOperationID("getProtectedUser").RequireFeatureAndPermission("users", "read").Produces(http.StatusOK)
	handler := app.MustBuild()

	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/users/550e8400-e29b-41d4-a716-446655440000", nil))
	if missing.Code != http.StatusUnauthorized || missing.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatalf("status=%d challenge=%q", missing.Code, missing.Header().Get("WWW-Authenticate"))
	}

	valid := httptest.NewRequest(http.MethodGet, "/users/550e8400-e29b-41d4-a716-446655440000", nil)
	valid.Header.Set("Authorization", "Bearer token")
	success := httptest.NewRecorder()
	handler.ServeHTTP(success, valid)
	if success.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", success.Code, success.Body.String())
	}
}

func TestJSONBodyFailuresUse413And415(t *testing.T) {
	app := New(Config{Info: Info{Title: "Users", Version: "1.0.0"}, JSONBodyLimit: 16})
	MapPut(app.Group(""), "/users/{id:uuid}", func(context.Context, updateInput) Result[userDTO] {
		return OK(userDTO{})
	}).WithOperationID("updateUserBody").WithValidation().Produces(http.StatusOK)
	handler := app.MustBuild()

	cases := []struct {
		name        string
		contentType string
		body        string
		status      int
	}{
		{name: "unsupported", contentType: "text/plain", body: `{}`, status: http.StatusUnsupportedMediaType},
		{name: "oversized", contentType: "application/json", body: `{"name":"this is too long"}`, status: http.StatusRequestEntityTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPut, "/users/550e8400-e29b-41d4-a716-446655440000", strings.NewReader(tc.body))
			request.Header.Set("Content-Type", tc.contentType)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != tc.status {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestOpenAPIOmitsCustomDialectAndDocumentsFrameworkFailures(t *testing.T) {
	app := New(Config{Info: Info{Title: "Users", Version: "1.0.0"}})
	MapPut(app.Group(""), "/users/{id:uuid}", func(context.Context, updateInput) Result[userDTO] {
		return OK(userDTO{})
	}).WithOperationID("updateUserDocumented").Produces(http.StatusOK)
	if err := app.MapOpenAPI("/openapi.json"); err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	app.MustBuild().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d", recorder.Code)
	}
	body := recorder.Body.String()
	if strings.Contains(body, "jsonSchemaDialect") {
		t.Fatalf("unexpected jsonSchemaDialect: %s", body)
	}
	for _, status := range []string{`"400"`, `"413"`, `"415"`, `"500"`} {
		if !strings.Contains(body, status) {
			t.Fatalf("missing response %s in %s", status, body)
		}
	}
}
