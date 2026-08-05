package oashttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDefaultRecoveryConvertsPanicToProblem(t *testing.T) {
	var reported error
	app := New(Config{
		Info: Info{Title: "Recovery", Version: "1.0.0"},
		ErrorHandler: func(_ context.Context, err error) {
			reported = err
		},
	})
	MapGet(app.Group(""), "/panic", func(context.Context, struct{}) Result[struct{}] {
		panic("boom")
	}).WithOperationID("panic").Produces(http.StatusOK)

	recorder := httptest.NewRecorder()
	app.MustBuild().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if reported == nil || !strings.Contains(reported.Error(), "boom") {
		t.Fatalf("reported=%v", reported)
	}
}

func TestRecoveryCoversUserMiddleware(t *testing.T) {
	app := New(Config{Info: Info{Title: "Recovery", Version: "1.0.0"}})
	if err := app.Use(func(http.Handler) http.Handler {
		return http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("middleware") })
	}); err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	app.MustBuild().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", recorder.Code)
	}
}

func TestRecoveryCanBeDisabled(t *testing.T) {
	app := New(Config{Info: Info{Title: "Recovery", Version: "1.0.0"}, DisablePanicRecovery: true})
	MapGet(app.Group(""), "/panic", func(context.Context, struct{}) Result[struct{}] {
		panic(errors.New("boom"))
	}).WithOperationID("panic").Produces(http.StatusOK)

	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("expected panic")
		}
	}()
	app.MustBuild().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/panic", nil))
}
