# Changelog

All notable changes to this project are documented here. The project follows Semantic Versioning.

## [Unreleased]

## [2.0.1] - 2026-08-11

### Added

- `MapHandler` and `RawOperationBuilder` for standard `net/http` handlers that share the typed API router, operation metadata, scoped middleware, security integration, panic recovery, duplicate-route checks, and OpenAPI document.
- Raw request media-type documentation through `.Consumes(...)`; raw handlers retain full responsibility for body parsing, streaming, limits, and `Content-Type` enforcement.
- OpenAPI registration for raw `HEAD`, `OPTIONS`, and `TRACE` operations in addition to the existing typed-operation methods.
- Runtime route-constraint validation for raw handlers, including `uuid`, integer, boolean, date, and datetime constraints.
- `Group.Use(...)` and operation-level `.Use(...)` middleware with deterministic parent-group, child-group, and operation ordering.
- `Config.SecurityProviders`, `SecurityProvider`, and `SecurityScheme` for named request-aware authentication and configurable OpenAPI `http` / `apiKey` schemes.
- `OperationInfo`, `OperationFromContext`, `OperationID`, and `RoutePattern` for low-cardinality metrics, traces, audit logging, and panic reporting.

### Changed

- Raw `.Consumes(...)` declares media types without claiming that the application-owned request body is mandatory.
- Default panic recovery now preserves `http.Flusher` and `http.Hijacker` when the underlying response writer provides them, so raw streaming and connection-upgrade handlers keep standard `net/http` capabilities.
- Named security providers validate OpenAPI component names and reject scheme fields that do not apply to the declared `http` or `apiKey` type.
- Operation metadata is now available to scoped middleware before the handler, to application-wide middleware after routing returns, and to `ErrorHandler` for recovered routed panics.
- Authenticated principals remain visible to scoped middleware and can also be observed by application-wide middleware after `next.ServeHTTP(...)` returns.
- `SECURITY.md` and `SUPPORT.md` now describe v2 as the supported feature line and v1 as security-fixes-only.
- Streaming and custom-representation endpoints can remain inside `oashttp` through `MapHandler` instead of requiring a separate external router.

### Compatibility

- All existing v2 typed endpoint, legacy `Authenticator` / `Authorizer`, failure formatting, and application-wide middleware APIs remain source-compatible.
- The reserved OpenAPI security scheme name `bearerAuth` remains owned by the legacy bearer integration.
- Named providers support OpenAPI `http` and `apiKey` schemes in v2.0.1; OAuth2/OpenID Connect flow objects and first-class scope requirements are intentionally deferred to a later release.
- No third-party runtime dependencies were added.

## [2.0.0] - 2026-08-06

### Changed

- Moved the canonical Go module to `github.com/sevlumen/oashttp/v2` after the repository transfer and rename.
- Updated internal imports, examples, badges, installation instructions, and repository links to the canonical location.

### Migration

Version 2 is a breaking module-path migration. Replace imports from `github.com/quang020102/go-osm` with `github.com/sevlumen/oashttp/v2`. Version `v1.0.1` remains available at the legacy module path.

## [1.0.1] - 2026-08-05

### Added

- `Config.FailureFormatter` for application-defined framework failure bodies, media types, and OpenAPI schemas.
- `Failure`, `Fail`, and `ProblemDetailsFormatter` public APIs.
- `ErrorJSON` for endpoint-specific JSON error envelopes.
- `ProducesResponse` for documenting caller-defined response schemas and media types.

### Changed

- Omit the OpenAPI `servers` field when `Config.Servers` is empty instead of injecting `Current server`.
- Route binding, validation, authentication, panic, nil-result, and serialization failures through the configured formatter.
- Register only the configured failure schema; custom formatters no longer expose the default `ProblemDetails` component.

### Compatibility

The default formatter preserves the `v1.0.0` Problem Details runtime and OpenAPI behavior. All new APIs are additive.

## [1.0.0] - 2026-08-05

### Added

- Stable typed `net/http` public API and `Version` constant.
- Default panic recovery with reporting through `Config.ErrorHandler`.
- RFC 6750-compatible `WWW-Authenticate` challenges for protected operations.
- Automatic OpenAPI documentation for framework-generated `400`, `401`, `403`, `413`, `415`, and `500` Problem Details responses.
- ETag and conditional GET support for the generated OpenAPI document.
- Hashed inline Swagger initializer CSP without `unsafe-inline`.
- CI coverage floor, race detection, route and binder fuzzing, benchmarks, golden-file verification, and `govulncheck`.
- Security, contribution, compatibility, and release documentation.

### Changed

- Omitted `jsonSchemaDialect` so OpenAPI 3.1 uses the default OAS dialect and Swagger UI does not warn about an unsupported custom dialect.
- JSON responses are serialized before response headers are committed, preventing a false success status when encoding fails.
- Oversized JSON requests now return `413`; unsupported body media types return `415`.
- Validation runs only after request binding succeeds.
- Framework-generated error responses use `application/problem+json`, `Cache-Control: no-store`, and `X-Content-Type-Options: nosniff`.

### Compatibility

The `v1` public API is considered stable. Breaking public API changes require a new major module version.
