package oashttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type benchmarkGetInput struct {
	ID UUID `path:"id"`
}
type benchmarkOutput struct {
	ID   UUID   `json:"id"`
	Name string `json:"name"`
}

func benchmarkHandler(b *testing.B) http.Handler {
	app := New(Config{Info: Info{Title: "Bench", Version: "1.0.0"}})
	group := app.Group("")
	MapGet(group, "/users/{id:uuid}", func(_ context.Context, input benchmarkGetInput) Result[benchmarkOutput] {
		return OK(benchmarkOutput{ID: input.ID, Name: "Alice"})
	}).WithOperationID("getBenchUser").Produces(http.StatusOK)
	MapPut(group, "/users/{id:uuid}", func(_ context.Context, input benchmarkInput) Result[benchmarkOutput] {
		return OK(benchmarkOutput{ID: input.ID, Name: input.Body.Name})
	}).WithOperationID("putBenchUser").WithValidation().Produces(http.StatusOK).ProducesProblem(http.StatusBadRequest)
	if err := app.MapOpenAPI("/openapi.json"); err != nil {
		b.Fatal(err)
	}
	handler, err := app.Build()
	if err != nil {
		b.Fatal(err)
	}
	return handler
}
func BenchmarkServeTypedGET(b *testing.B) {
	handler := benchmarkHandler(b)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/users/550e8400-e29b-41d4-a716-446655440000", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != 200 {
			b.Fatal(rec.Code)
		}
	}
}
func BenchmarkServeTypedPUTJSON(b *testing.B) {
	handler := benchmarkHandler(b)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPut, "/users/550e8400-e29b-41d4-a716-446655440000?page=1", strings.NewReader(`{"name":"Alice","age":30}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != 200 {
			b.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
	}
}
func BenchmarkServeOpenAPIJSON(b *testing.B) {
	handler := benchmarkHandler(b)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != 200 {
			b.Fatal(rec.Code)
		}
	}
}
