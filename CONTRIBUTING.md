# Contributing

## Requirements

- Go 1.22 or newer.
- No third-party runtime dependencies in `go.mod`.
- Preserve the package boundaries documented in [`docs/architecture.md`](docs/architecture.md).

## Local quality gates

Run before submitting a change:

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
govulncheck ./...
```

The total statement coverage must remain at or above 70%. Changes to generated OpenAPI output must intentionally update `testdata/users.openapi.golden.json`.

## Architecture

The root `oashttp` package owns the public API and final composition. Internal execution is split across focused packages for operation compilation, routing, binding, validation, security, failure handling, schema generation, OpenAPI modeling, and documentation serving.

`internal/operation.Definition` is the shared operation IR for runtime and OpenAPI behavior. Keep `internal/core` dependency-light and free of subsystem orchestration. See [`docs/architecture.md`](docs/architecture.md) before introducing new cross-package dependencies.

## Compatibility

Public identifiers in the root `oashttp` package are covered by the major-version-2 compatibility promise. Prefer additive changes. Breaking changes require a new major module path.
