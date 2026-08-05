# Changelog

All notable changes to this project are documented here. The project follows Semantic Versioning.

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
- JSON responses are serialized before headers are committed, preventing a false success status when encoding fails.
- Oversized JSON requests now return `413`; unsupported body media types return `415`.
- Validation runs only after request binding succeeds.
- Framework-generated error responses use `application/problem+json`, `Cache-Control: no-store`, and `X-Content-Type-Options: nosniff`.

### Compatibility

The `v1` public API is considered stable. Breaking public API changes require a new major module version.
