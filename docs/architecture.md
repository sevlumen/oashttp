# Architecture

`oashttp` keeps a small public facade in the root package and implements runtime behavior behind `internal/*` packages. The design goal is to keep one operation definition as the source of truth for both request execution and generated OpenAPI while preventing HTTP, schema, security, and failure concerns from collapsing into one package.

## Dependency direction

```text
public oashttp facade
        |
        v
internal/operation
   |      |        |        |
   v      v        v        v
 route  binding  validation security
          |          |       |
          v          v       v
        schema  validationrule failure
          |
          v
         oas31

result + operation/openapi
          |
          v
       httpsem

internal/core
= dependency-light shared contracts and request-scoped state
```

This diagram is conceptual rather than a complete Go import graph. The important constraint is that orchestration flows from the root facade and `internal/operation` toward focused implementation packages, not back toward the public facade.

## Package responsibilities

### Root `oashttp`

Owns the stable public API and application composition:

- `App`, `Config`, and `Group` lifecycle;
- typed and raw operation registration;
- public builders and aliases;
- application middleware composition;
- OpenAPI and Swagger endpoint registration;
- final wiring of compiled operations into `http.ServeMux`.

The root package should orchestrate subsystems rather than reimplement binding, validation, security, schema, or OpenAPI internals.

### `internal/operation`

Owns the operation intermediate representation and compilation pipeline.

`Definition` is the shared source of truth for typed and raw operations. Compilation is intentionally split by responsibility:

- `validate.go` validates definitions and compiles request plans;
- `runtime.go` builds runtime HTTP execution, security, middleware, binding/validation, and raw-path validation;
- `openapi.go` builds the corresponding OpenAPI operation;
- `compile.go` coordinates those pieces and returns the compiled runtime/OpenAPI pair.

Runtime and OpenAPI behavior must continue to originate from the same `Definition`.

### `internal/core`

Contains dependency-light contracts and request-scoped state shared by internal subsystems, including principals, validators, failure/result contracts, operation metadata, and the request carrier.

`internal/core` must not become an orchestration or business-logic package. It should remain independent of sibling internal packages.

### `internal/route`

Owns route parsing, normalized route patterns, and route constraints.

### `internal/binding`

Owns typed request binding and compiled binding plans.

### `internal/validation`

Owns validation-plan traversal and runtime execution. It consumes normalized rule semantics from `internal/validationrule` rather than parsing validation tags independently.

### `internal/validationrule`

Owns validation tag syntax, type compatibility, and the dependency-light normalized rule representation shared by runtime validation and schema generation. It must not execute validation, generate OpenAPI types, or depend on sibling internal packages.

### `internal/httpsem`

Owns dependency-light HTTP protocol facts that must remain identical across runtime and compilation. Response-body eligibility for informational responses, `204`, `205`, and `304` is defined here and consumed by both result writing and OpenAPI response generation.

### `internal/security`

Owns authentication/authorization execution and principal propagation. Public security contracts remain defined through `internal/core` and re-exported by the root facade.

### `internal/failure`

Owns resolution and serialization of framework failures.

### `internal/schema`

Owns Go-type reflection and reusable OpenAPI schema registration. Validation-derived schema constraints consume the normalized rules from `internal/validationrule` so generated contracts match runtime validation semantics.

### `internal/oas31`

Owns the internal OpenAPI 3.1 document model and serialization. It must not know about application configuration, security-provider execution, or runtime handlers.

### `internal/docs`

Owns HTTP handlers for serving generated OpenAPI documents and Swagger UI.

## Architecture rules

1. The root package owns public API stability and final composition.
2. `internal/operation.Definition` remains the single operation IR used by runtime and OpenAPI compilation.
3. `internal/operation/compile.go` is an orchestrator; runtime and OpenAPI details belong in focused files.
4. `internal/core`, `internal/oas31`, `internal/validationrule`, and `internal/httpsem` are dependency leaves with narrowly defined responsibilities.
5. Runtime validation and schema/OpenAPI generation must consume the same normalized validation-rule semantics rather than parsing supported validation tags independently.
6. HTTP protocol facts shared by runtime and generated contracts must originate from the narrowest shared internal boundary, not duplicated helper logic.
7. Binding, validation, security, failure, route, schema, and OpenAPI packages must not import the root `oashttp` package.
8. Configuration mutation is supported only before `Build`; synchronization must make concurrent configuration calls race-free and preserve the freeze boundary.
9. Existing typed and raw behavior must remain source-compatible within major version 2.
10. Runtime dependencies remain standard-library-only unless a future compatibility decision explicitly changes that policy.
11. No production package may depend on `examples/` or test-only code.

## Change guidance

When adding a feature, place the contract at the narrowest stable boundary and keep execution in the focused subsystem. Prefer extending `Definition` only when both runtime behavior and OpenAPI documentation need the same operation-level fact.

Avoid creating new packages solely to reduce file length. Split files inside an existing package when responsibilities are related and package boundaries would add dependency churn without a meaningful abstraction. A new dependency-leaf package is appropriate when one protocol or semantic fact must be consumed consistently by otherwise independent subsystems.

Any architecture refactor must preserve the existing quality gates: Go-version matrix, formatting, module consistency, vet, repeated tests, race detector, coverage floor, OpenAPI golden verification, fuzzing, benchmarks, and vulnerability scanning.
