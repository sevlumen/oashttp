package oashttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestErrorHandlerSeesOperationMetadataForRecoveredPanic(t *testing.T) {
	seenOperationID := ""
	app := New(Config{
		Info: Info{Title: "Recovery metadata", Version: "1.0.0"},
		ErrorHandler: func(ctx context.Context, _ error) {
			seenOperationID = OperationID(ctx)
		},
	})
	MapGet(app.Group(""), "/panic", func(context.Context, struct{}) Result[map[string]bool] {
		panic("boom")
	}).WithOperationID("panicOperation").Produces(http.StatusOK)

	recorder := httptest.NewRecorder()
	app.MustBuild().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if seenOperationID != "panicOperation" {
		t.Fatalf("operation id=%q", seenOperationID)
	}
}
