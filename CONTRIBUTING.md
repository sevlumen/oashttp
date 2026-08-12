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
bash scripts/check-public-api.sh
```

The total statement coverage must remain at or above 70%. Changes to generated OpenAPI output must intentionally update `testdata/users.openapi.golden.json`.

The public-API compatibility check requires the repository's stable release tags and network access for its pinned Go tool. CI checks out full tag history for this gate.

## Architecture

The root `oashttp` package owns the public API and final composition. Internal execution is split across focused packages for operation compilation, routing, binding, validation, security, failure handling, schema generation, OpenAPI modeling, and documentation serving.

`internal/operation.Definition` is the shared operation IR for runtime and OpenAPI behavior. Keep `internal/core` dependency-light and free of subsystem orchestration. See [`docs/architecture.md`](docs/architecture.md) before introducing new cross-package dependencies.

## Compatibility

Public identifiers in the root `oashttp` package are covered by the major-version-2 compatibility promise. CI compares the current public API with the latest stable v2 release. Backward-incompatible public API changes cannot be released within v2 and require a new major module path.

Backward-compatible exported API additions require a minor release. Patch releases must not add exported API. The exported `Version` constant is the sole value-normalization exception used during comparison: its release string may change, but the symbol must remain an exported constant with the same public shape.

The public-API diff gate protects source compatibility. Behavioral compatibility still depends on focused tests, integration tests, and review.

## Release process

A release is published only from source that is already merged to `main` and has passed the normal `main` CI run.

1. Make `version.go`, `README.md`, and `CHANGELOG.md` agree on the release version.
2. Merge the release-ready source to `main` and require the push-triggered `main` CI run to pass.
3. Create `publish/vX.Y.Z` from that exact verified `main` commit without adding release-only commits.
4. The release workflow verifies the publish branch still equals `main`, verifies `version.go`, reruns the release quality gates, checks the proposed version against the latest stable v2 public API, creates tag `vX.Y.Z`, and publishes the GitHub Release.
5. Never reuse, move, or force-update an existing release tag.

The `publish/vX.Y.Z` branch is a release control ref, not a development branch. If any verification step fails, fix the source through the normal PR flow and create a new publish ref from the corrected `main` commit.
