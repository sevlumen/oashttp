# Public API Compatibility Gate Design

## Goal

Turn the existing major-version-2 compatibility promise into a machine-enforced contract before adding more public capabilities to `oashttp`.

This work protects the already released `github.com/sevlumen/oashttp/v2` API without changing runtime behavior, adding a runtime dependency, or publishing a new version by itself.

## Current state

`oashttp` documents the root package as the stable public facade and `CONTRIBUTING.md` states that public identifiers in the root package are covered by the v2 compatibility promise. The current CI verifies Go 1.22-1.26, formatting, zero runtime dependencies, module consistency, vet, repeated tests, race safety, coverage, OpenAPI golden output, fuzz targets, benchmarks, and vulnerability scanning, but it does not compare the exported API against the last stable release.

The current stable baseline is `v2.0.2`.

## Design principles

1. Backward-incompatible public API changes must fail CI before merge.
2. Backward-compatible additive API changes are allowed during development, but release-time SemVer must reflect them.
3. Patch releases must not silently introduce public API additions that require a minor version.
4. Consumer-facing compile/use contracts must be tested from an external package, not only from `package oashttp` tests.
5. Compatibility tooling must not become a runtime or `go.mod` dependency.
6. The gate must be deterministic: tool versions are pinned and the comparison baseline is an explicit stable v2 tag.
7. The gate is an additional safety net, not a claim that API-diff tooling can detect behavioral compatibility.

## Compatibility tool

Use the Go project's `golang.org/x/exp/cmd/gorelease` tool, pinned to:

```text
golang.org/x/exp/cmd/gorelease@v0.0.0-20260727155853-b88d891fe743
```

The pin corresponds to `golang.org/x/exp` commit `b88d891fe7432eb5d8ba7549e2a6f7ea098074ff`.

`gorelease` analyzes public API differences using `apidiff`, can compare the current checkout with a base module version, reports compatible and incompatible API changes, and validates whether a proposed release version is consistent with SemVer. It is experimental tooling, so pinning is mandatory; `@latest` is not allowed in repository workflows.

The tool is installed only inside CI/release jobs with `go install`. It must not be added to `go.mod`, `go.sum`, or production code.

## Stable baseline selection

The comparison baseline is the highest stable v2 tag reachable from the repository history, matching exactly:

```text
^v2\.[0-9]+\.[0-9]+$
```

Pre-release tags are excluded.

For the first implementation, `v2.0.2` is expected to be selected. Future runs must derive the baseline dynamically rather than hard-coding `v2.0.2`, so APIs added in later stable v2 releases are protected by subsequent PRs.

The workflow should fetch tags and derive the highest stable v2 tag using Git version ordering. If no stable v2 baseline exists, the gate fails closed; that state would contradict the repository's current release history.

## Pull-request compatibility gate

Add a dedicated CI step/job that runs on the PR checkout after normal module validation:

```text
stable v2 tag
     |
     v
pinned gorelease -base=<stable-tag>
     |
     +-- incompatible API change --> fail
     |
     +-- compatible addition --> pass and report suggested minor version
     |
     +-- no public API change --> pass
```

The PR gate does not pass `-version`. Its purpose is to reject incompatible v2 API changes while allowing compatible additions to be reviewed normally.

Any tool execution error, inability to resolve the stable baseline, package load failure, or `gorelease` unsuccessful result fails CI. There is no `|| true` or warning-only mode.

### Module-level scope

`gorelease` compares non-`internal` packages in the module. The intended compatibility promise remains the public `oashttp` facade.

The repository currently also contains `examples/users-api`, which is `package main`. `gorelease` does not compare the exported API of matched `main` packages, so ordinary edits to that example do not become public API contracts. A future removal of that package path may be conservatively reported as a module-level incompatibility. If that false positive becomes relevant, it must be addressed explicitly in a later tooling change rather than weakening or bypassing the gate in an unrelated PR.

No current production package may be moved out of `internal/` as part of this work.

## Release-time SemVer gate

Extend the guarded release workflow with the same pinned `gorelease` version.

Before `Publish GitHub Release`, after the existing identity/version/main-SHA checks and normal quality gates, derive the latest stable v2 baseline lower than the proposed release tag and run:

```text
gorelease -base=<stable-v2-base> -version=<proposed-vX.Y.Z>
```

Expected behavior:

- no public API change: patch or minor release is acceptable;
- backward-compatible public API addition: a patch release is rejected and a minor release is required;
- backward-incompatible API change: any v2 release is rejected;
- invalid/reused/non-canonical version: release remains blocked.

The existing release workflow remains responsible for source identity, version consistency, quality gates, tag creation, and GitHub Release publication. The new SemVer check is inserted before publication and does not replace those gates.

## External consumer contract tests

Add black-box tests using:

```go
package oashttp_test
```

These tests import the module exactly as a downstream consumer does and may use only exported identifiers.

They are intentionally not an exhaustive API snapshot; `gorelease` provides broad exported-API comparison. The black-box tests protect high-value integration shapes and verify that the public facade remains practically consumable.

### Consumer flows to cover

At minimum, cover these public flows:

1. Construct `Config`/`Info`, call `New`, and build an application.
2. Create nested groups and attach public middleware.
3. Register at least one typed endpoint using the public generic mapping API and operation builder chain.
4. Exercise public `Result` helpers/types through a typed handler.
5. Register a raw `http.Handler` through the public raw-handler integration point.
6. Configure the public authentication/security contracts with a consumer-defined implementation.
7. Register OpenAPI and Swagger UI endpoints using only exported configuration types.
8. Call `Build`/`MustBuild` and serve representative requests through `net/http/httptest` where useful.

The tests should assert behavior only where it strengthens the compile/use contract. They must not duplicate existing deep runtime tests or access unexported state.

## Test strategy

Implementation follows a characterization-first/TDD sequence.

### Gate characterization

1. Add the pinned compatibility command to a feature branch that is API-identical to `v2.0.2`; it must pass.
2. Create a temporary test-only incompatible public API mutation on the feature branch, such as changing/removing an exported signature, and confirm the compatibility command fails for the intended reason.
3. Remove the temporary mutation completely before the production commit.
4. Create a temporary compatible exported addition and confirm the PR-mode check remains successful while reporting a minor-version suggestion.
5. Remove the temporary addition completely before final integration.

These characterization mutations are verification scaffolding only and must not appear in the final PR diff.

### Release SemVer characterization

Verify the release command against controlled temporary version/API states:

- unchanged API + patch proposal: success;
- compatible API addition + patch proposal: failure;
- compatible API addition + minor proposal: success;
- incompatible API change + v2 proposal: failure.

Again, temporary API changes must not remain in the final branch.

### Consumer tests

Add external-package tests and require them to compile/run under the existing Go 1.22-1.26 matrix.

## CI placement

The normal CI continues to run all existing gates.

The compatibility check should run on Ubuntu with the repository's newest CI Go version unless tool compatibility requires a separately documented version. It is acceptable to install the pinned tool in the quality job or in a dedicated `Public API compatibility` job; a dedicated job is preferred if it keeps failure output easy to identify without duplicating expensive quality steps.

The job must use `actions/checkout@v6` with enough history/tags to identify the stable baseline. The checkout/tag-fetch behavior must be explicit rather than assuming a shallow clone contains release tags.

## Dependency policy

The repository's runtime/module dependency policy remains unchanged:

- `go.mod` continues to contain no third-party requirements;
- `go.sum` remains absent/unchanged unless existing repository policy changes separately;
- compatibility tools are ephemeral CI tools installed at pinned versions;
- application/runtime code does not import `golang.org/x/exp`.

The existing zero-runtime-dependency CI check must continue to pass unchanged.

## Documentation changes

Update `CONTRIBUTING.md` to state that:

- PRs are checked against the latest stable v2 API;
- incompatible public API changes require a new major module path and cannot be released in v2;
- compatible public API additions require a minor release;
- patch releases may contain fixes with no visible API additions;
- the release workflow enforces the proposed version against the API diff.

If necessary, add a short architecture note that the root facade's compatibility promise is machine-enforced. Do not expand README feature documentation for this tooling-only change.

## Error handling and failure policy

The compatibility gate fails closed for:

- missing/unresolvable stable baseline;
- failure to install the pinned tool;
- package loading errors;
- incompatible public API changes;
- release SemVer mismatch;
- unexpected tool diagnostics that make the result unsafe to interpret.

A compatibility failure may only be resolved by changing the source/API or, for legitimate additive API work, choosing the correct minor release. CI must not contain an allowlist that silently suppresses incompatible changes.

If the experimental tool itself becomes unusable in the future, replacing/pinning a new compatibility mechanism is a dedicated maintenance change with its own review and characterization tests.

## Platform limitation

The pinned Go tooling currently evaluates packages in the CI build configuration and does not provide a complete cross-`GOOS`/`GOARCH` compatibility proof. `oashttp` currently has no production `//go:build` files, so one Ubuntu/amd64 compatibility pass is sufficient for this phase.

If platform-specific public source is introduced later, CI must expand compatibility checks to each supported API configuration rather than assuming this gate covers them automatically.

## Non-goals

This change does not:

- add CORS, OAuth/OIDC, request IDs, or other runtime features;
- redesign or rename existing public APIs;
- change operation/schema/runtime semantics;
- bump `Version` or publish a release solely for adding CI protection;
- promise behavioral compatibility beyond what tests and review establish;
- make examples or internal packages part of the documented public facade.

## Definition of Done

This P1 item is complete when:

1. a pinned public-API compatibility gate runs on PR CI against the latest stable v2 release;
2. RED/green characterization proves it rejects a representative incompatible API change;
3. a compatible additive change is accepted by PR-mode checking;
4. the release workflow validates proposed SemVer against the same stable API baseline before publication;
5. release characterization proves patch/minor/incompatible cases behave as designed;
6. black-box `package oashttp_test` consumer contracts compile/run across Go 1.22-1.26;
7. existing formatting, zero-runtime-dependency, module, vet, repeated test, race, coverage, OpenAPI golden, fuzz, benchmark, and vulnerability gates remain green;
8. no runtime/public API behavior is changed by the final diff;
9. `CONTRIBUTING.md` documents the enforced compatibility/release policy;
10. the implementation is reviewed and merged through the normal PR flow.

Because this item adds only compatibility infrastructure and tests, it does not require a new `oashttp` release by itself. A later source release will automatically be subject to the new release-time SemVer gate.
