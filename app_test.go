package oashttp

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"
)

func TestNewAppliesZeroDependencyDefaults(t *testing.T) {
	app := New(Config{Info: Info{Title: "Users API", Version: "1.0.0"}})
	if app.config.OpenAPIVersion != "3.1.0" {
		t.Fatalf("OpenAPIVersion = %q", app.config.OpenAPIVersion)
	}
	if app.config.JSONBodyLimit != 1<<20 {
		t.Fatalf("JSONBodyLimit = %d", app.config.JSONBodyLimit)
	}
	if !app.config.DisallowUnknownJSONFields {
		t.Fatal("unknown JSON fields must be rejected by default")
	}
	if len(app.config.Servers) != 1 || app.config.Servers[0].URL != "/" {
		t.Fatalf("Servers = %#v", app.config.Servers)
	}
}

func TestBuildFreezesApplicationAndIsIdempotent(t *testing.T) {
	app := New(Config{Info: Info{Title: "Users API", Version: "1.0.0"}})
	first, firstErr := app.Build()
	second, secondErr := app.Build()
	if firstErr != nil || secondErr != nil {
		t.Fatalf("Build errors = %v, %v", firstErr, secondErr)
	}
	if reflect.ValueOf(first).Pointer() != reflect.ValueOf(second).Pointer() {
		t.Fatal("repeated Build must return the same handler")
	}
	err := app.Use(func(next http.Handler) http.Handler { return next })
	if !errors.Is(err, ErrFrozen) {
		t.Fatalf("Use error = %v", err)
	}
}

func TestFluentMutationAfterBuildPanicsWithErrFrozen(t *testing.T) {
	app := New(Config{Info: Info{Title: "Users API", Version: "1.0.0"}})
	builder := MapGet(app.Group(""), "/health", func(context.Context, struct{}) Result[struct{}] { return OK(struct{}{}) }).WithOperationID("health").Produces(http.StatusOK)
	if _, err := app.Build(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if recovered := recover(); !errors.Is(asError(recovered), ErrFrozen) {
			t.Fatalf("panic=%v", recovered)
		}
	}()
	builder.WithSummary("changed")
}
func TestConflictingServeMuxPatternsReturnBuildError(t *testing.T) {
	app := New(Config{Info: Info{Title: "Users API", Version: "1.0.0"}})
	group := app.Group("")
	type firstInput struct {
		ID string `path:"id"`
	}
	type secondInput struct {
		Name string `path:"name"`
	}
	MapGet(group, "/users/{id}", func(context.Context, firstInput) Result[struct{}] { return OK(struct{}{}) }).WithOperationID("first").Produces(http.StatusOK)
	MapGet(group, "/users/{name}", func(context.Context, secondInput) Result[struct{}] { return OK(struct{}{}) }).WithOperationID("second").Produces(http.StatusOK)
	if _, err := app.Build(); err == nil {
		t.Fatal("expected conflicting route error")
	}
}
func asError(value any) error {
	if err, ok := value.(error); ok {
		return err
	}
	return nil
}
