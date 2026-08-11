# oashttp

[![CI](https://github.com/sevlumen/oashttp/actions/workflows/ci.yml/badge.svg)](https://github.com/sevlumen/oashttp/actions/workflows/ci.yml)

`oashttp` is a zero-third-party-runtime-dependency Go library for typed `net/http` JSON endpoints, compiled request binding and validation, RFC 9457-style Problem Details, security integration, panic recovery, scoped middleware, raw-handler escape hatches, and OpenAPI 3.1 generation.

**Stable release:** `v2.0.1`

**Module:** `github.com/sevlumen/oashttp/v2`

**Minimum Go version:** Go 1.22

## Install

```bash
go get github.com/sevlumen/oashttp/v2@v2.0.1
```

## What's new in v2.0.1

Version 2.0.1 adds backward-compatible production integration boundaries while preserving the typed JSON API:

- `MapHandler` for standard `http.Handler` endpoints that stay in the same router and OpenAPI document;
- `Group.Use` and operation-level `.Use(...)` middleware;
- named request-aware `SecurityProvider` implementations and configurable OpenAPI `http` / `apiKey` security schemes;
- request-scoped `OperationInfo`, `OperationID`, and `RoutePattern` metadata for metrics, traces, audit logging, and error reporting;
- corrected v2/v1 security and compatibility policy documentation.

OAuth2 flows and first-class scope requirements are not part of this patch; the named-security-provider API is the foundation for that work.

## Migrating from v1

Version 2 uses the canonical module path `github.com/sevlumen/oashttp/v2`. Replace imports from `github.com/quang020102/go-osm`, then run:

```bash
go get github.com/sevlumen/oashttp/v2@v2.0.1
go mod tidy
```

The legacy `github.com/quang020102/go-osm@v1.0.1` module remains available for applications that are not ready to migrate. The v1 line receives security fixes only; new feature development targets v2.

The Go module has no third-party runtime dependencies. Swagger UI assets are loaded by the browser from a pinned CDN by default and can be replaced with an application-controlled mirror.

## Quick start

```go
package main

import (
    "context"
    "log"
    "net/http"
    "time"

    oashttp "github.com/sevlumen/oashttp/v2"
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

Top-level typed input types are non-pointer structs. Exported fields can bind from:

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

Call `.WithValidation()` to compile validation rules once at build time. The current v2 API supports:

`required`, `min`, `max`, `len`, `email`, `uuid`, `e164`, `oneof`, `gte`, and `lte`.

Application-specific validation can be added with `Config.Validator`.

## Raw `net/http` handlers

Use `MapHandler` when an endpoint needs streaming, multipart processing, redirects, HTML, or another representation that should remain under application control.

```go
uploads := app.Group("/v1")

upload := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    objectID := r.PathValue("id")
    _ = objectID

    // Stream r.Body directly to application-owned storage code.
    w.WriteHeader(http.StatusNoContent)
})

oashttp.MapHandler(uploads, http.MethodPost, "/objects/{id:uuid}", upload).
    WithOperationID("uploadObject").
    WithTags("Storage").
    Consumes("application/octet-stream").
    Produces(http.StatusNoContent)
```

Raw handlers:

- share the same `http.ServeMux`, duplicate-route detection, panic recovery, scoped middleware, security, operation metadata, and OpenAPI document as typed operations;
- receive the original `*http.Request` and control body reading, streaming, response headers, status codes, and representations;
- enforce declared route constraints such as `{id:uuid}` before the user handler runs;
- use `.Consumes(...)` only to document request media types in OpenAPI; the raw handler remains responsible for `Content-Type`, body-size, multipart, and streaming policy;
- can use `.ProducesResponse(...)` when a response has a schema that should appear in OpenAPI.

`MapHandler` is an escape hatch, not a multipart or storage framework.

## Scoped middleware

`App.Use` remains application-wide. Groups and individual operations can add narrower middleware:

```go
admin := app.Group("/admin")
_ = admin.Use(AdminAuditMiddleware)

operation := oashttp.MapGet(admin, "/status", handler).
    WithOperationID("adminStatus").
    Produces(http.StatusOK)

operation.Use(OperationAuditMiddleware)
```

Nested groups inherit the middleware that existed on their parent when the child group was created. Middleware attached after an operation is registered does not retroactively affect that operation.

For an operation, the effective request order is:

```text
panic recovery
application middleware
routing + operation metadata
authentication / authorization
parent group middleware
child group middleware
operation middleware
typed binding + validation OR raw path-constraint validation
handler
```

Scoped middleware therefore sees the authenticated principal and operation metadata before calling the handler.

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

Handlers can return the global format with `Fail`, `BadRequest`, `NotFound`, and the other existing failure helpers. For an endpoint-specific JSON body, pair `ErrorJSON` with `ProducesResponse`.

Generated typed OpenAPI operations automatically document applicable `400`, `401`, `403`, `413`, `415`, and `500` framework responses. Raw operations document only the framework failures that the raw integration itself can generate plus explicitly declared responses.

## Security integration

### Legacy bearer integration

The existing `Authenticator` / `Authorizer` API remains supported and backward-compatible:

```go
type Authenticator interface {
    Authenticate(ctx context.Context, token string) (*oashttp.Principal, error)
}

operation.RequireFeatureAndPermission("core.users", "core.users.update")
```

When no custom `Authorizer` is configured, exact feature and permission membership is required. Legacy authentication failures return RFC 6750-compatible `WWW-Authenticate: Bearer` challenges.

### Named security providers

For schemes that need request-aware credential extraction, configure named providers:

```go
type ClientKeyProvider struct{}

func (ClientKeyProvider) SecurityScheme() oashttp.SecurityScheme {
    return oashttp.SecurityScheme{
        Type: "apiKey",
        Name: "X-API-Key",
        In:   "header",
    }
}

func (ClientKeyProvider) Authenticate(ctx context.Context, r *http.Request) (*oashttp.Principal, error) {
    if r.Header.Get("X-API-Key") == "" {
        return nil, oashttp.ErrUnauthorized
    }
    return &oashttp.Principal{Subject: "client-1"}, nil
}

app := oashttp.New(oashttp.Config{
    Info: oashttp.Info{Title: "Storage API", Version: "1.0.0"},
    SecurityProviders: map[string]oashttp.SecurityProvider{
        "clientKey": ClientKeyProvider{},
    },
})

operation.RequireSecurity("clientKey")
```

The current named-provider OpenAPI model supports `type: http` and `type: apiKey`. The reserved name `bearerAuth` remains owned by the legacy `Config.Authenticator` integration. OAuth2/OpenID Connect flow objects and first-class scopes are planned separately.

## Operation metadata and observability

Every routed operation exposes stable metadata:

```go
info, ok := oashttp.OperationFromContext(ctx)
operationID := oashttp.OperationID(ctx)
route := oashttp.RoutePattern(ctx)
```

`OperationInfo` contains the operation ID, HTTP method, and normalized OpenAPI route pattern. This avoids using concrete request URLs as metric labels.

Scoped group/operation middleware and handlers can read metadata before the handler executes. Application-wide middleware wraps routing, so it can read the populated metadata and authenticated principal after `next.ServeHTTP(...)` returns. The same request-scoped metadata is available to `ErrorHandler` when the default recovery layer reports a panic from a routed operation.

This supports metrics such as:

```text
http_server_duration{operation="createClient",status="200"}
```

without high-cardinality labels derived from resource IDs.

## Panic recovery

Recovery is enabled by default. Recovered panics are reported through `Config.ErrorHandler`; when no response has been committed, the client receives a generic `500` Problem Details response. Set `Config.DisablePanicRecovery` only when another outer server layer provides equivalent recovery.

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

`oashttp` is production-oriented for typed JSON APIs with controlled raw-handler escape hatches. The application remains responsible for:

- TLS termination and trusted proxy configuration;
- server timeouts, graceful shutdown, and connection limits;
- JWT/OAuth/OIDC verification and key management;
- rate limiting, abuse controls, CORS, and CSRF protection;
- logs, metrics, traces, exporters, and alerting;
- database transactions and business authorization policy;
- streaming limits, multipart policy, storage I/O, and other raw-handler protocol behavior.

The library provides integration points rather than implementing those application concerns itself.

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
