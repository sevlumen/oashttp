package oashttp

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/oashttp/oashttp/internal/binding"
	"github.com/oashttp/oashttp/internal/route"
	"github.com/oashttp/oashttp/internal/validation"
)

type benchmarkBody struct {
	Name string `json:"name" validate:"required,min=2"`
	Age  int    `json:"age" validate:"gte=18"`
}
type benchmarkInput struct {
	ID   UUID          `path:"id"`
	Page int           `query:"page"`
	Body benchmarkBody `body:"json"`
}

func BenchmarkBindCompiledInput(b *testing.B) {
	pattern, _ := route.Parse("/users/{id:uuid}")
	plan, _ := binding.Compile(reflect.TypeOf(benchmarkInput{}), pattern, binding.Options{JSONBodyLimit: 1 << 20, DisallowUnknownJSONFields: true})
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPut, "/users/550e8400-e29b-41d4-a716-446655440000?page=1", strings.NewReader(`{"name":"Alice","age":30}`))
		req.SetPathValue("id", "550e8400-e29b-41d4-a716-446655440000")
		req.Header.Set("Content-Type", "application/json")
		_, errs := plan.Bind(req)
		if len(errs) != 0 {
			b.Fatal(errs)
		}
	}
}
func BenchmarkValidateCompiledPlan(b *testing.B) {
	plan, _ := validation.Compile(reflect.TypeOf(benchmarkBody{}))
	value := benchmarkBody{Name: "Alice", Age: 30}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if errs := plan.Validate(value); len(errs) != 0 {
			b.Fatal(errs)
		}
	}
}
