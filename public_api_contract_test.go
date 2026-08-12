package oashttp_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	oashttp "github.com/sevlumen/oashttp/v2"
)

type contractInput struct {
	ID oashttp.UUID `path:"id"`
}

type contractOutput struct {
	ID oashttp.UUID `json:"id"`
}

type apiKeyProvider struct{}

func (apiKeyProvider) SecurityScheme() oashttp.SecurityScheme {
	return oashttp.SecurityScheme{Type: "apiKey", Name: "X-API-Key", In: "header"}
}

func (apiKeyProvider) Authenticate(_ context.Context, r *http.Request) (*oashttp.Principal, error) {
	return &oashttp.Principal{Subject: r.Header.Get("X-API-Key")}, nil
}

func TestPublicFacadeConsumerContract(t *testing.T) {
	if oashttp.Version == "" {
		t.Fatal("Version must remain exported")
	}

	app := oashttp.New(oashttp.Config{
		Info:    oashttp.Info{Title: "Contract API", Version: "1.0.0"},
		Servers: []oashttp.Server{{URL: "https://api.example.test"}},
		SecurityProviders: map[string]oashttp.SecurityProvider{
			"apiKey": apiKeyProvider{},
		},
	})

	middleware := oashttp.Middleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Contract", "1")
			next.ServeHTTP(w, r)
		})
	})
	if err := app.Use(middleware); err != nil {
		t.Fatal(err)
	}

	group := app.Group("/api").Group("/users")
	if err := group.Use(middleware); err != nil {
		t.Fatal(err)
	}

	oashttp.MapGet(group, "/{id:uuid}", func(ctx context.Context, in contractInput) oashttp.Result[contractOutput] {
		if oashttp.OperationID(ctx) != "getContractUser" {
			t.Fatalf("operation id=%q", oashttp.OperationID(ctx))
		}
		if oashttp.RoutePattern(ctx) == "" {
			t.Fatal("route pattern must be visible")
		}
		info, ok := oashttp.OperationFromContext(ctx)
		if !ok || info.Method != http.MethodGet {
			t.Fatalf("operation info=%+v ok=%v", info, ok)
		}
		return oashttp.OK(contractOutput{ID: in.ID}).WithHeader("X-Result", "1")
	}).
		WithOperationID("getContractUser").
		WithValidation().
		WithTags("Contract").
		WithSummary("Get contract user").
		WithDescription("Consumer contract").
		Use(middleware).
		Produces(http.StatusOK).
		ProducesProblem(http.StatusBadRequest)

	oashttp.MapGet(group, "/secured/{id:uuid}", func(_ context.Context, in contractInput) oashttp.Result[contractOutput] {
		return oashttp.Accepted(contractOutput{ID: in.ID})
	}).
		WithOperationID("getSecuredContractUser").
		RequireSecurity("apiKey").
		Produces(http.StatusAccepted).
		ProducesProblem(http.StatusUnauthorized)

	oashttp.MapHandler(group, http.MethodPost, "/raw", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).
		WithOperationID("postRawContract").
		WithTags("Contract").
		Consumes("application/octet-stream").
		Produces(http.StatusNoContent)

	if err := app.MapOpenAPI("/openapi.json"); err != nil {
		t.Fatal(err)
	}
	if err := app.MapSwaggerUI("/swagger", oashttp.SwaggerUIConfig{DocumentURL: "/openapi.json"}); err != nil {
		t.Fatal(err)
	}

	handler, err := app.Build()
	if err != nil {
		t.Fatal(err)
	}
	if app.MustBuild() == nil {
		t.Fatal("MustBuild returned nil")
	}

	id := "550e8400-e29b-41d4-a716-446655440000"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/users/"+id, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), id) {
		t.Fatalf("body=%s", rec.Body.String())
	}
	if rec.Header().Get("X-Contract") != "1" || rec.Header().Get("X-Result") != "1" {
		t.Fatalf("headers=%v", rec.Header())
	}

	docs := httptest.NewRecorder()
	handler.ServeHTTP(docs, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	if docs.Code != http.StatusOK {
		t.Fatalf("openapi status=%d", docs.Code)
	}
}
