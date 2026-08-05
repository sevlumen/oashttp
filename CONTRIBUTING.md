# Contributing

## Requirements

- Go 1.22 or newer.
- No third-party runtime dependencies in `go.mod`.

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
```

The total statement coverage must remain at or above 70%. Changes to generated OpenAPI output must intentionally update `testdata/users.openapi.golden.json`.

## Compatibility

Public identifiers in the root `oashttp` package are covered by the `v1` compatibility promise. Prefer additive changes. Breaking changes require a new major module path.
