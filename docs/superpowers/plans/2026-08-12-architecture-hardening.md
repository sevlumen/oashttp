# oashttp Architecture Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce internal coupling, remove configuration-phase races, and make application/operation compilation easier to maintain without changing the public API or documented runtime contracts.

**Architecture:** Keep the root `oashttp` package as the public facade and `internal/*` as implementation. `internal/operation.Definition` remains the single operation IR, while validation/request-plan compilation, runtime compilation, and OpenAPI compilation are separated by responsibility. `App.Build()` remains the composition root.

**Tech Stack:** Go 1.22+, standard library `net/http`, OpenAPI 3.1, zero third-party runtime dependencies.

## Global Constraints

- No breaking public API changes.
- Preserve typed endpoint behavior and `MapHandler` contracts.
- Preserve OpenAPI output unless a test-backed correctness fix requires a change.
- Keep minimum Go version at 1.22.
- Keep zero third-party runtime dependencies.
- Keep `internal/operation.Definition` as the shared runtime/OpenAPI IR.
- Preserve the existing CI gates: Go 1.22-1.26, formatting, module consistency, vet, repeated tests, race detector, coverage >= 70%, OpenAPI golden verification, fuzzing, benchmarks, and `govulncheck`.

---

### Task 1: Make group configuration race-safe

**Files:**
- Modify: `group.go`
- Modify: `map.go`
- Modify: `raw.go`
- Create: `group_registration.go`
- Create: `group_concurrency_test.go`

**Interfaces:**
- Preserve `Group.Use`, `Group.Group`, typed map functions, and `MapHandler` signatures.
- Add internal `(*App).registerGroupOperation(*Group, *internaloperation.Definition)`.

- [x] Add a concurrent configuration regression test.
- [x] Snapshot child-group middleware under `App.mu`.
- [x] Snapshot operation middleware and append the operation under the same `App.mu` critical section.
- [x] Route typed and raw registration through the synchronized helper.
- [x] Verify with `go test -race ./...`.

### Task 2: Split operation compiler responsibilities

**Files:**
- Modify: `internal/operation/compile.go`
- Create: `internal/operation/validate.go`
- Create: `internal/operation/runtime.go`
- Create: `internal/operation/openapi.go`

**Interfaces:**
- Preserve `Compile(def *Definition, opts Options) (Compiled, error)`.
- Preserve `Options`, `Compiled`, errors, runtime behavior, middleware order, security behavior, and generated OpenAPI.

- [x] Keep `compile.go` as orchestration only.
- [x] Move definition validation and request-plan compilation to `validate.go`.
- [x] Move runtime HTTP/security/middleware execution to `runtime.go`.
- [x] Move OpenAPI operation generation to `openapi.go`.
- [x] Verify `go test ./internal/operation -count=3` through the repeated CI suite.
- [x] Verify full repeated and race suites.

### Task 3: Keep App.Build focused on composition

**Files:**
- Modify: `app.go`
- Create: `openapi_build.go`
- Create: `openapi_build_test.go`

**Interfaces:**
- Preserve `App.Build`, `App.MustBuild`, and public configuration behavior.
- Keep OpenAPI security validation and HTTP Path Item method behavior unchanged.

- [x] Move OpenAPI document construction into `newOpenAPIDocument`.
- [x] Move compiled-operation application into `applyCompiledOperation`.
- [x] Move security-scheme and Path Item helpers out of `app.go`.
- [x] Add direct helper tests for HTTP methods and security-scheme field validation.
- [x] Verify OpenAPI golden output is unchanged.

### Task 4: Document architecture boundaries

**Files:**
- Create: `docs/architecture.md`
- Create: `internal/core/doc.go`
- Modify: `CONTRIBUTING.md`

- [x] Document package responsibilities and dependency direction.
- [x] Define `internal/core` as a dependency-light contract/request-state kernel.
- [x] Link architecture guidance from contribution documentation.
- [x] Correct the stale contribution compatibility wording from v1 to major version 2.

### Task 5: Final architecture regression gate

- [x] `test -z "$(gofmt -l .)"`
- [x] zero runtime dependency check
- [x] `go mod tidy` with no module diff
- [x] `go vet ./...`
- [x] `go test ./... -shuffle=on -count=3`
- [x] `go test -race ./...`
- [x] total statement coverage >= 70% (71.4% in CI run #161)
- [x] OpenAPI golden verification with no diff
- [x] route fuzz target for 15 seconds
- [x] binding fuzz target for 15 seconds
- [x] benchmarks
- [x] `govulncheck ./...` (no known vulnerabilities in CI run #161)
- [x] review final diff against `main` for accidental public API or scope changes

## Publication Boundary

Implementation work belongs on `refactor/v2-architecture-hardening`. Open a single draft PR only after explicit authorization. Do not merge, tag, or publish a release without separate authorization.
