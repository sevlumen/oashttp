# oashttp

[![CI](https://github.com/quang020102/go-osm/actions/workflows/ci.yml/badge.svg)](https://github.com/quang020102/go-osm/actions/workflows/ci.yml)

`oashttp` is a zero-third-party-runtime-dependency Go library for typed `net/http` JSON endpoints, compiled request binding and validation, RFC 9457-style Problem Details, authorization hooks, panic recovery, and OpenAPI 3.1 generation.

**Stable release:** `v1.0.1`

**Module:** `github.com/quang020102/go-osm`

**Minimum Go version:** Go 1.22

## Install

```bash
go get github.com/quang020102/go-osm@v1.0.1
```

The Go module has no third-party runtime dependencies. Swagger UI assets are loaded by the browser from a pinned CDN by default and can be replaced with an application-controlled mirror.

## Quick start

```go
package main

import (
    "context"
    "log"
    "net/http"
    "time"

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
        ErrorHandler: func(_ context.Context, err error) {
            log.Printf("request failure: %v", err)
        },
    })

    users := app.Group("/api/v1").Group("/users")
    oashttp.MapGet(users, "/{id:uuid}", func(_ context.Context, input GetUserInput) oashttp.Result[UserDTO] {
        return oashttp.OK(UserDTO{ID: input.ID})
    }).
        WithOperationID("getUser").
        WithTags("Users").
        WithSummary("Get user").
        Produces(http.StatusOK)

    if err := app.MapOpenAPI("/openapi.json"); err != nil {
        log.Fatal(err)
    }
    if err := app.MapSwaggerUI("/swagger", oashttp.SwaggerUIConfig{DocumentURL: "/openapi.json"}); err != nil {
        log.Fatal(err)
    }

    server := &http.Server{
        Addr:              ":8080",
        Handler:           app.MustBuild(),
        ReadHeaderTimeout: 5 * time.Second,
        ReadTimeout:       15 * time.Second,
        WriteTimeout:      30 * time.Second,
        IdleTimeout:       60 * time.Second,
        MaxHeaderBytes:    1 << 20,
    }
    log.Fatal(server.ListenAndServe())
}
```

Open `/openapi.json` for the generated document and `/swagger` for Swagger UI. When `Config.Servers` is empty, the generated document omits `servers`; OpenAPI clients still resolve requests against the current origin. Configure `Servers` only when an explicit environment URL should appear.

## Binding

Top-level input types are non-pointer structs. Exported fields can bind from:

```go
type UpdateInput struct {
    ID      oashttp.UUID `path:"id"`
    DryRun  bool         `query:"dryRun"`
    TraceID string       `header:"X-Trace-ID"`
    Body    UpdateBody   `body:"json"`
}
```

Supported route constraints are `string`, `uuid`, `int`, `int64`, `bool`, `date`, and `datetime`.

JSON bodies:

- are limited to 1 MiB by default;
- require `Content-Type: application/json` when non-empty;
- reject unknown fields by default;
- must contain exactly one JSON value;
- return `413` when over the configured limit;
- return `415` for an unsupported media type.

Use `Config.JSONBodyLimit` and `Config.AllowUnknownJSONFields` to change the defaults.

## Validation

Call `.WithValidation()` to compile validation rules once at build time. Version 1 supports:

`required`, `min`, `max`, `len`, `email`, `uuid`, `e164`, `oneof`, `gte`, and `lte`.

Application-specific validation can be added with `Config.Validator`.

## Results and errors

Success helpers include `OK`, `Created`, `Accepted`, `NoContent`, and `JSON(status, value)`. JSON is serialized before response headers are committed, so an unsupported output value returns a formatted `500 RESPONSE_SERIALIZATION_FAILED` response instead of a false `2xx`.

By default, framework failures use RFC 9457-style `ProblemDetails` with `application/problem+json`, `Cache-Control: no-store`, and `X-Content-Type-Options: nosniff`. `ProducesProblem(status)` documents the configured global failure schema.

Applications can replace the public failure envelope while keeping framework binding, validation, authentication, panic, and serialization failures consistent:

```go
type APIError struct {
    Success bool `json:"success"`
    Error struct {
        Code    string              `json:"code"`
        Message string              `json:"message"`
        Fields  map[string][]string `json:"fields,omitempty"`
    } `json:"error"`
}

type APIErrorFormatter struct{}

func (APIErrorFormatter) ContentType() string { return "application/json" }
func (APIErrorFormatter) Model() any          { return APIError{} }
func (APIErrorFormatter) Format(f oashttp.Failure) any {
    value := APIError{}
    value.Error.Code = f.Code
    value.Error.Message = f.Detail
    value.Error.Fields = f.Errors
    return value
}

app := oashttp.New(oashttp.Config{
    Info:             oashttp.Info{Title: "Users API", Version: "1.0.0"},
    FailureFormatter: APIErrorFormatter{},
})
```

Handlers can return the global format with `Fail`, `BadRequest`, `NotFound`, and the other existing failure helpers. For an endpoint-specific JSON body, pair `ErrorJSON` with `ProducesResponse`:

```go
return oashttp.ErrorJSON[UserDTO](http.StatusConflict, APIError{/* ... */})

operation.ProducesResponse(
    http.StatusConflict,
    "User already exists",
    "application/json",
    APIError{},
)
```

Generated OpenAPI operations automatically document applicable `400`, `401`, `403`, `413`, `415`, and `500` responses using the configured failure model.

## Panic recovery

An outer recovery middleware is enabled by default. Recovered panics are reported through `Config.ErrorHandler`; when no response has been committed, the client receives a generic `500` Problem Details response. Set `Config.DisablePanicRecovery` only when another outer server layer provides equivalent recovery.

## Security integration

Protected operations use application-provided interfaces. `oashttp` deliberately does not parse JWTs or validate signatures.

```go
type Authenticator interface {
    Authenticate(ctx context.Context, token string) (*oashttp.Principal, error)
}
```

Protect an operation with:

```go
operation.RequireFeatureAndPermission("core.users", "core.users.update")
```

When no custom `Authorizer` is configured, exact feature and permission membership is required. Authentication failures return RFC 6750-compatible `WWW-Authenticate: Bearer` challenges.

## OpenAPI and Swagger UI

The generated document uses OpenAPI 3.1 and omits `jsonSchemaDialect`, which selects the default OpenAPI Schema dialect and avoids the Swagger UI custom-dialect warning.

The OpenAPI endpoint supports `GET`, `HEAD`, ETag-based conditional requests, and short public caching. Swagger's inline initializer is authorized by a SHA-256 CSP hash; the CSP does not require `unsafe-inline`.

The default browser assets are pinned to `swagger-ui-dist@5.32.11` on jsDelivr. Override both URLs for an internal mirror:

```go
_ = app.MapSwaggerUI("/swagger", oashttp.SwaggerUIConfig{
    DocumentURL: "/openapi.json",
    CSSURL:      "https://docs.example.internal/swagger-ui.css",
    BundleURL:   "https://docs.example.internal/swagger-ui-bundle.js",
})
```

## Production boundaries

`oashttp` is production-oriented for typed JSON APIs. The application remains responsible for:

- TLS termination and trusted proxy configuration;
- server timeouts, graceful shutdown, and connection limits;
- JWT/OAuth/OIDC implementation and key management;
- rate limiting, abuse controls, CORS, and CSRF protection;
- logs, metrics, traces, and alerting;
- database transactions and business authorization policy.

Endpoints requiring `application/x-www-form-urlencoded`, browser redirects, HTML, streaming, WebSockets, or custom representations should use standard `net/http` handlers alongside the typed JSON API.

## Quality gates

```bash
gofmt -w .
go mod tidy
go vet ./...
go test ./... -shuffle=on -count=3
go test -race ./...
go test ./... -coverprofile=coverage.out
go test ./internal/route -run=^$ -fuzz=FuzzRouteParserNeverPanics -fuzztime=15s
go test ./internal/binding -run=^$ -fuzz=FuzzCompiledBinderNeverPanics -fuzztime=15s
go test ./... -bench=. -run=^$ -benchtime=100ms
```

CI enforces a total statement coverage floor of 70%, verifies the golden OpenAPI document, runs `govulncheck`, and tests Go 1.22 through Go 1.26.

See `examples/users-api`, `SECURITY.md`, `SUPPORT.md`, `CONTRIBUTING.md`, and `CHANGELOG.md`.
