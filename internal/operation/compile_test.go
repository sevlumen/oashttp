package operation

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/sevlumen/oashttp/v2/internal/binding"
	"github.com/sevlumen/oashttp/v2/internal/core"
	"github.com/sevlumen/oashttp/v2/internal/schema"
)

type compileInput struct {
	ID string `path:"id"`
}
type compileOutput struct {
	ID string `json:"id"`
}
type compileResult struct{}

func (compileResult) WriteHTTPWithFailureFormatter(w http.ResponseWriter, _ func(error), _ core.FailureFormatter, _ string) {
	w.WriteHeader(http.StatusOK)
}

func TestCompileBuildsRuntimeAndOpenAPIOperation(t *testing.T) {
	def := &Definition{Method: http.MethodGet, UserRoute: "/users/{id:uuid}", FullRoute: "/users/{id:uuid}", InputType: reflect.TypeOf(compileInput{}), OutputType: reflect.TypeOf(compileOutput{}), OperationID: "getUser", Responses: map[int]ResponseSpec{http.StatusOK: {Kind: ResponseJSON, Description: "OK"}}, Invoke: func(context.Context, reflect.Value) core.ResultWriter { return compileResult{} }}
	compiled, err := Compile(def, Options{Binding: binding.Options{}, Registry: schema.NewRegistry()})
	if err != nil {
		t.Fatal(err)
	}
	if compiled.Pattern.ServeMuxPath != "/users/{id}" {
		t.Fatalf("pattern=%#v", compiled.Pattern)
	}
	request := httptest.NewRequest(http.MethodGet, "/users/not-a-uuid", nil)
	request.SetPathValue("id", "not-a-uuid")
	recorder := httptest.NewRecorder()
	compiled.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if compiled.Operation.OperationID != "getUser" {
		t.Fatalf("operation=%#v", compiled.Operation)
	}
}

func TestCompileProtectedOperationRequiresAuthenticator(t *testing.T) {
	def := &Definition{Method: http.MethodGet, UserRoute: "/users/{id}", FullRoute: "/users/{id}", InputType: reflect.TypeOf(compileInput{}), OutputType: reflect.TypeOf(compileOutput{}), OperationID: "getUser", Feature: "users", Permission: "read", Responses: map[int]ResponseSpec{http.StatusOK: {Kind: ResponseJSON}}, Invoke: func(context.Context, reflect.Value) core.ResultWriter { return compileResult{} }}
	if _, err := Compile(def, Options{Registry: schema.NewRegistry()}); err == nil {
		t.Fatal("expected authenticator error")
	}
}

type bodyInput struct {
	Body struct {
		Name string `json:"name" validate:"required"`
	} `body:"json"`
}

type validatorFunc func(context.Context, any) []core.FieldError

func (f validatorFunc) Validate(ctx context.Context, value any) []core.FieldError {
	return f(ctx, value)
}

type authFunc func(context.Context, string) (*core.Principal, error)

func (f authFunc) Authenticate(ctx context.Context, token string) (*core.Principal, error) {
	return f(ctx, token)
}

func validDefinition() *Definition {
	return &Definition{
		Method:      http.MethodGet,
		UserRoute:   "/users/{id:uuid}",
		FullRoute:   "/users/{id:uuid}",
		InputType:   reflect.TypeOf(compileInput{}),
		OutputType:  reflect.TypeOf(compileOutput{}),
		OperationID: "getUser",
		Responses:   map[int]ResponseSpec{http.StatusOK: {Kind: ResponseJSON, Description: "OK"}},
		Invoke:      func(context.Context, reflect.Value) core.ResultWriter { return compileResult{} },
	}
}

func TestCompileRejectsInvalidDefinitions(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Definition)
	}{
		{name: "missing operation id", mutate: func(d *Definition) { d.OperationID = "" }},
		{name: "partial security", mutate: func(d *Definition) { d.Feature = "users" }},
		{name: "invalid response", mutate: func(d *Definition) { d.Responses[999] = ResponseSpec{} }},
		{name: "invalid route", mutate: func(d *Definition) { d.FullRoute = "users" }},
		{name: "missing success response", mutate: func(d *Definition) {
			d.Responses = map[int]ResponseSpec{http.StatusBadRequest: {Kind: ResponseProblem}}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			def := validDefinition()
			tc.mutate(def)
			if _, err := Compile(def, Options{Registry: schema.NewRegistry()}); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestCompileDocumentsFrameworkProblemResponses(t *testing.T) {
	def := &Definition{
		Method:      http.MethodPost,
		UserRoute:   "/users",
		FullRoute:   "/users",
		InputType:   reflect.TypeOf(bodyInput{}),
		OutputType:  reflect.TypeOf(compileOutput{}),
		OperationID: "createUser",
		Responses:   map[int]ResponseSpec{http.StatusCreated: {Kind: ResponseJSON}},
		Invoke:      func(context.Context, reflect.Value) core.ResultWriter { return compileResult{} },
	}
	compiled, err := Compile(def, Options{Registry: schema.NewRegistry()})
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"400", "413", "415", "500"} {
		response, ok := compiled.Operation.Responses[status]
		if !ok || response.Content["application/problem+json"].Schema == nil {
			t.Fatalf("response %s=%#v", status, response)
		}
	}
	if compiled.Operation.RequestBody == nil || !compiled.Operation.RequestBody.Required {
		t.Fatalf("request body=%#v", compiled.Operation.RequestBody)
	}
}

func TestCompiledHandlerStopsBeforeValidationWhenBindingFails(t *testing.T) {
	called := false
	def := &Definition{
		Method:      http.MethodPost,
		UserRoute:   "/users",
		FullRoute:   "/users",
		InputType:   reflect.TypeOf(bodyInput{}),
		OutputType:  reflect.TypeOf(compileOutput{}),
		OperationID: "createUser",
		Validation:  true,
		Responses:   map[int]ResponseSpec{http.StatusOK: {Kind: ResponseJSON}},
		Invoke:      func(context.Context, reflect.Value) core.ResultWriter { called = true; return compileResult{} },
	}
	compiled, err := Compile(def, Options{
		Registry: schema.NewRegistry(),
		Validator: validatorFunc(func(context.Context, any) []core.FieldError {
			t.Fatal("custom validator must not run after binding failure")
			return nil
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":`))
	request.Header.Set("Content-Type", "application/json")
	compiled.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest || called {
		t.Fatalf("status=%d called=%v body=%s", recorder.Code, called, recorder.Body.String())
	}
}

func TestCompiledHandlerUsesCustomValidationAndNilResultGuard(t *testing.T) {
	def := validDefinition()
	def.Validation = true
	compiled, err := Compile(def, Options{
		Registry: schema.NewRegistry(),
		Validator: validatorFunc(func(context.Context, any) []core.FieldError {
			return []core.FieldError{{Location: "body", Field: "name", Messages: []string{"rejected"}}}
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/users/550e8400-e29b-41d4-a716-446655440000", nil)
	request.SetPathValue("id", "550e8400-e29b-41d4-a716-446655440000")
	recorder := httptest.NewRecorder()
	compiled.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	def = validDefinition()
	def.Invoke = func(context.Context, reflect.Value) core.ResultWriter { return nil }
	compiled, err = Compile(def, Options{Registry: schema.NewRegistry()})
	if err != nil {
		t.Fatal(err)
	}
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/users/550e8400-e29b-41d4-a716-446655440000", nil)
	request.SetPathValue("id", "550e8400-e29b-41d4-a716-446655440000")
	compiled.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestProtectedCompileAndRuntimeFailures(t *testing.T) {
	def := validDefinition()
	def.Feature = "users"
	def.Permission = "read"
	if _, err := Compile(def, Options{Registry: schema.NewRegistry()}); err == nil {
		t.Fatal("expected missing authenticator error")
	}
	compiled, err := Compile(def, Options{
		Registry: schema.NewRegistry(),
		Authenticator: authFunc(func(context.Context, string) (*core.Principal, error) {
			return nil, fmt.Errorf("invalid")
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/users/550e8400-e29b-41d4-a716-446655440000", nil)
	request.Header.Set("Authorization", "Bearer bad")
	compiled.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized || recorder.Header().Get("WWW-Authenticate") != `Bearer error="invalid_token"` {
		t.Fatalf("status=%d challenge=%q", recorder.Code, recorder.Header().Get("WWW-Authenticate"))
	}
}

func TestCompileRejectsInvalidCustomResponses(t *testing.T) {
	tests := []struct {
		name string
		spec ResponseSpec
	}{
		{
			name: "nil model",
			spec: ResponseSpec{Kind: ResponseCustom, ContentType: "application/json"},
		},
		{
			name: "invalid content type",
			spec: ResponseSpec{Kind: ResponseCustom, ContentType: "not a media type", ModelType: reflect.TypeOf(compileOutput{})},
		},
		{
			name: "unsupported model",
			spec: ResponseSpec{Kind: ResponseCustom, ContentType: "application/json", ModelType: reflect.TypeOf(make(chan int))},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			def := validDefinition()
			def.Responses[http.StatusConflict] = tc.spec
			if _, err := Compile(def, Options{Registry: schema.NewRegistry()}); err == nil {
				t.Fatal("expected custom response build error")
			}
		})
	}
}
