package operation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/oashttp/oashttp/internal/binding"
	"github.com/oashttp/oashttp/internal/core"
	"github.com/oashttp/oashttp/internal/schema"
)

type compileInput struct {
	ID string `path:"id"`
}
type compileOutput struct {
	ID string `json:"id"`
}
type compileResult struct{}

func (compileResult) WriteHTTP(w http.ResponseWriter, _ func(error)) { w.WriteHeader(http.StatusOK) }

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
