# oashttp

`oashttp` is a zero-third-party-dependency Go library for typed `net/http` endpoints, compiled request binding and validation, RFC 9457-style Problem Details, and OpenAPI 3.1 JSON generation.

> The module path is `github.com/quang020102/go-osm` and the minimum supported Go version is 1.22.

## Install

```bash
go get github.com/quang020102/go-osm
```

The core module has zero third-party Go dependencies.
Swagger UI is loaded by the browser from a pinned CDN URL.
The API server does not download or execute frontend assets.

## Quick start

```go
package main

import (
    "context"
    "net/http"

    oashttp "github.com/quang020102/go-osm"
)

type GetUserInput struct {
    ID oashttp.UUID `path:"id"`
}

type UserDTO struct {
    ID oashttp.UUID `json:"id"`
}

func main() {
    app := oashttp.New(oashttp.Config{
        Info: oashttp.Info{Title: "Users API", Version: "1.0.0"},
    })

    users := app.Group("/api/v1").Group("/users")
    oashttp.MapGet(users, "/{id:uuid}", func(_ context.Context, input GetUserInput) oashttp.Result[UserDTO] {
        return oashttp.OK(UserDTO{ID: input.ID})
    }).
        WithOperationID("getUser").
        WithTags("Users").
        WithSummary("Get user").
        Produces(http.StatusOK).
        ProducesProblem(http.StatusBadRequest)

    _ = app.MapOpenAPI("/openapi.json")
    _ = app.MapSwaggerUI("/swagger", oashttp.SwaggerUIConfig{
        DocumentURL: "/openapi.json",
    })

    _ = http.ListenAndServe(":8080", app.MustBuild())
}
```

Open:

- `http://localhost:8080/openapi.json`
- `http://localhost:8080/swagger`

The default OpenAPI server is relative (`/`), so Swagger calls the same origin, hostname, port, and reverse-proxy entry point used to load the page.

## Binding

Top-level input types are structs. Exported fields can bind from:

```go
type UpdateInput struct {
    ID      oashttp.UUID `path:"id"`
    DryRun  bool         `query:"dryRun"`
    TraceID string       `header:"X-Trace-ID"`
    Body    UpdateBody   `body:"json"`
}
```

Supported route constraints are `string`, `uuid`, `int`, `int64`, `bool`, `date`, and `datetime`.

JSON request bodies are limited to 1 MiB by default, reject unknown fields by default, and must contain exactly one JSON value. Set `Config.JSONBodyLimit` or `Config.AllowUnknownJSONFields` to override those defaults.

## Validation

Call `.WithValidation()` to compile and execute validation tags. Version 1 supports:

`required`, `min`, `max`, `len`, `email`, `uuid`, `e164`, `oneof`, `gte`, and `lte`.

Application-specific validation can be added through `Config.Validator` without introducing a dependency into the library.

## Security

Protected operations use application-provided interfaces. `oashttp` does not parse JWTs or validate signatures itself.

```go
type Authenticator interface {
    Authenticate(ctx context.Context, token string) (*oashttp.Principal, error)
}
```

Use:

```go
operation.RequireFeatureAndPermission("core.users", "core.users.update")
```

When no custom `Authorizer` is configured, exact feature and permission membership is required on the authenticated principal.

## Private Swagger asset mirror

The default browser assets are pinned to `swagger-ui-dist@5.32.11` on jsDelivr. Override both URLs for an internal mirror:

```go
_ = app.MapSwaggerUI("/swagger", oashttp.SwaggerUIConfig{
    DocumentURL: "/openapi.json",
    CSSURL:      "https://docs.example.internal/swagger-ui.css",
    BundleURL:   "https://docs.example.internal/swagger-ui-bundle.js",
})
```

No network access is required to build, test, or run the API server. Only a browser loading the Swagger page contacts the configured asset origin.

## Development gates

```bash
go mod tidy
go list -m all
go vet ./...
go test ./...
go test -race ./...
go test ./... -bench=. -run=^$ -benchtime=100ms
```

`go list -m all` must print only `github.com/quang020102/go-osm`.

See `examples/users-api` for authentication, validation, Problem Details, golden OpenAPI, and Swagger UI coverage.
