package benchmarks

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	oashttp "github.com/sevlumen/oashttp/v2"
)

func BenchmarkHTTPCoreTypedNoBody(b *testing.B) {
	app := oashttp.New(oashttp.Config{Info: oashttp.Info{Title: "bench", Version: "test"}})
	oashttp.MapGet(app.Group(""), "/health", func(context.Context, struct{}) oashttp.Result[struct{}] {
		return oashttp.NoContent[struct{}]()
	}).WithOperationID("health").Produces(http.StatusNoContent)
	handler := app.MustBuild()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request.Clone(request.Context()))
	}
}

func BenchmarkHTTPCoreRawNoBody(b *testing.B) {
	app := oashttp.New(oashttp.Config{Info: oashttp.Info{Title: "bench", Version: "test"}})
	oashttp.MapHandler(app.Group(""), http.MethodGet, "/health", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).WithOperationID("rawHealth").Produces(http.StatusNoContent)
	handler := app.MustBuild()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request.Clone(request.Context()))
	}
}

func BenchmarkHTTPCoreTypedJSON(b *testing.B) {
	app := oashttp.New(oashttp.Config{Info: oashttp.Info{Title: "bench", Version: "test"}})
	oashttp.MapGet(app.Group(""), "/json", func(context.Context, struct{}) oashttp.Result[map[string]string] {
		return oashttp.OK(map[string]string{"status": "ok"})
	}).WithOperationID("json").Produces(http.StatusOK)
	handler := app.MustBuild()
	request := httptest.NewRequest(http.MethodGet, "/json", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request.Clone(request.Context()))
	}
}
