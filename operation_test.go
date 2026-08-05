package oashttp

import (
	"context"
	"net/http"
	"net/http/httptest"
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
