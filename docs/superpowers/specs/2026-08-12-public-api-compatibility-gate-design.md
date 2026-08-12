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
7. The exported `Version` constant has one narrow compatibility exception: its release-number value may change, but its exported identifier/kind/type contract may not.
8. The gate is an additional safety net, not a claim that API-diff tooling can detect behavioral compatibility.

## Compatibility tool

Use the Go project's `golang.org/x/exp/cmd/gorelease` tool, pinned to:

```text
golang.org/x/exp/cmd/gorelease@v0.0.0-20260727155853-b88d891fe743
```

The pin corresponds to `golang.org/x/exp` commit `b88d891fe7432eb5d8ba7549e2a6f7ea098074ff`.

`gorelease` analyzes public API differences using `apidiff`, can compare the current checkout with a base module version, reports compatible and incompatible API changes, and validates whether a proposed release version is consistent with SemVer. It is experimental tooling, so pinning is mandatory; `@latest` is not allowed in repository workflows.

The tool is executed ephemerally with an explicit module version. It must not be added to `go.mod`, `go.sum`, or production code.

## Stable baseline selection

The comparison baseline is the highest stable v2 tag reachable from the repository history, matching exactly:

```text
^v2\.[0-9]+\.[0-9]+$
```

Pre-release tags are excluded.

For the first implementation, `v2.0.2` is expected to be selected. Future runs must derive the baseline dynamically rather than hard-coding `v2.0.2`, so APIs added in later stable v2 releases are protected by subsequent PRs.

The compatibility helper fetches/uses repository tags and derives the highest stable v2 tag with Git version ordering. If no stable v2 baseline exists, the gate fails closed; that state would contradict the repository's current release history.

For release checking, the baseline must also be lower than the proposed release version. The proposed tag itself does not exist yet under the existing guarded release workflow.

## `Version` constant exception

`oashttp.Version` is an exported constant whose value intentionally changes on every release. Generic Go API-diff rules treat an exported constant value change as incompatible because constant values can participate in compile-time expressions. Applying that rule literally to this release metadata constant would make every future v2 patch/minor release impossible.

The compatibility gate therefore ignores only the release-number **value change** of this one symbol while continuing to protect its public shape.

### Normalization mechanism

Create one repository helper, for example:

```text
scripts/check-public-api.sh
```

The helper:

1. resolves the stable v2 baseline;
2. reads the baseline `version.go` and requires the expected declaration shape for `const Version`;
3. creates a temporary source tree from the current `HEAD` using `git archive`, outside the working repository and without `.git` metadata;
4. requires the current `version.go` to contain the same expected `const Version` declaration shape;
5. replaces only the current release-number string with the baseline `Version` value inside the temporary source copy;
6. runs pinned `gorelease` from that temporary module directory;
7. deletes the temporary tree on exit.

This normalization exists only for comparison. It never modifies the checked-out branch, commit, tag, or release source.

If `Version` is removed, renamed, changed from a constant, changes declaration shape unexpectedly, or cannot be normalized exactly once, the helper fails closed. No other exported constant/value is normalized or suppressed.

The final source and release tag continue to contain the real proposed `Version` value and remain subject to the existing version/release identity checks.

## Pull-request compatibility gate

Add a dedicated CI job that runs on the PR checkout after normal module validation:

```text
stable v2 tag
     |
     v
temporary HEAD source copy
     |
normalize only Version value
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

Any helper/tool execution error, inability to resolve the stable baseline, package load failure, normalization ambiguity, or `gorelease` unsuccessful result fails CI. There is no `|| true` or warning-only mode.

### Module-level scope

`gorelease` compares non-`internal` packages in the module. The intended compatibility promise remains the public `oashttp` facade.

The repository currently also contains `examples/users-api`, which is `package main`. `gorelease` does not compare the exported API of matched `main` packages, so ordinary edits to that example do not become public API contracts. A future removal of that package path may be conservatively reported as a module-level incompatibility. If that false positive becomes relevant, it must be addressed explicitly in a dedicated compatibility-tooling change rather than bypassing the gate in an unrelated PR.

No current production package may be moved out of `internal/` as part of this work.

## Release-time SemVer gate

Extend the guarded release workflow by calling the same compatibility helper.

Before `Publish GitHub Release`, after the existing identity/version/main-SHA checks and normal quality gates, derive the latest stable v2 baseline lower than the proposed release tag and run the equivalent of:

```text
gorelease -base=<stable-v2-base> -version=<proposed-vX.Y.Z>
```

against the temporary source copy with only the `Version` value normalized.

Expected behavior:

- no public API change other than the normalized `Version` value: patch or minor release is acceptable;
- backward-compatible public API addition: a patch release is rejected and a minor release is required;
- backward-incompatible API change: any v2 release is rejected;
- invalid/reused/non-canonical version: release remains blocked by existing and/or new release checks.

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
8. Reference `oashttp.Version` from the external package so removal/visibility changes are caught independently of normalization.
9. Call `Build`/`MustBuild` and serve representative requests through `net/http/httptest` where useful.

The tests should assert behavior only where it strengthens the compile/use contract. They must not duplicate existing deep runtime tests or access unexported state.

## Test strategy

Implementation follows a characterization-first/TDD sequence.

### Gate characterization

1. Add the pinned compatibility helper to a feature branch that is API-identical to `v2.0.2`; it must pass.
2. Create a temporary incompatible public API mutation on the feature branch, such as changing/removing an exported signature, and confirm the compatibility helper fails for the intended reason.
3. Remove the temporary incompatible mutation completely.
4. Create a temporary compatible exported addition and confirm PR-mode checking remains successful while reporting a minor-version suggestion.
5. Remove the temporary addition completely.
6. Temporarily change only the `Version` release string and confirm PR-mode checking remains successful because normalization removes only that expected metadata difference.
7. Temporarily remove or change the declaration kind of `Version` and confirm the helper fails closed.
8. Restore the real branch source before final integration.

Characterization mutations are verification scaffolding only and must not appear in the final PR diff.

### Release SemVer characterization

Verify the release helper against controlled temporary version/API states:

- unchanged API except `Version` value + patch proposal: success;
- compatible API addition + patch proposal: failure;
- compatible API addition + minor proposal: success;
- incompatible API change + v2 proposal: failure;
- malformed/missing `Version` declaration: failure before API comparison.

Temporary source/API mutations must not remain in the final branch.

### Consumer tests

Add external-package tests and require them to compile/run under the existing Go 1.22-1.26 matrix.

## CI placement

The normal CI continues to run all existing gates.

Add a dedicated `Public API compatibility` job so failure output remains easy to identify and expensive quality gates are not duplicated. The compatibility job uses the repository's newest CI Go version, currently Go 1.26, because the pinned experimental tooling is not part of the library's Go 1.22 consumer support contract.

The job uses `actions/checkout@v6` with full tag/history availability (`fetch-depth: 0` or an equivalent explicit tag fetch) so baseline selection never depends on shallow-checkout behavior.

The job calls the repository helper rather than duplicating baseline/normalization logic in YAML.

## Dependency policy

The repository's runtime/module dependency policy remains unchanged:

- `go.mod` continues to contain no third-party requirements;
- `go.sum` remains absent/unchanged under the current policy;
- compatibility tooling is executed ephemerally at an exact pinned version;
- application/runtime code does not import `golang.org/x/exp`.

The existing zero-runtime-dependency CI check must continue to pass unchanged.

## Documentation changes

Update `CONTRIBUTING.md` to state that:

- PRs are checked against the latest stable v2 API;
- `Version` is the sole documented value-normalization exception and its symbol/declaration remains protected;
- incompatible public API changes require a new major module path and cannot be released in v2;
- compatible public API additions require a minor release;
- patch releases may contain fixes with no visible API additions;
- the release workflow enforces the proposed version against the API diff.

Add one short note to `docs/architecture.md` that the root facade's compatibility promise is machine-enforced by CI/release gates. Do not expand README feature documentation for this tooling-only change.

## Error handling and failure policy

The compatibility gate fails closed for:

- missing/unresolvable stable baseline;
- failure to execute/download the pinned tool;
- package loading errors;
- ambiguous or failed `Version` normalization;
- incompatible public API changes;
- release SemVer mismatch;
- unexpected tool diagnostics that make the result unsafe to interpret.

A compatibility failure may only be resolved by changing the source/API or, for legitimate additive API work, choosing the correct minor release. CI must not contain a general allowlist that silently suppresses incompatible changes.

If the experimental tool itself becomes unusable in the future, replacing/pinning a new compatibility mechanism is a dedicated maintenance change with its own review and characterization tests.

## Platform limitation

The pinned Go tooling evaluates packages in the CI build configuration and does not provide a complete cross-`GOOS`/`GOARCH` compatibility proof. `oashttp` currently has no production `//go:build` files, so one Ubuntu/amd64 compatibility pass is sufficient for this phase.

If platform-specific public source is introduced later, CI must expand compatibility checks to each supported API configuration rather than assuming this gate covers them automatically.

## Non-goals

This change does not:

- add CORS, OAuth/OIDC, request IDs, or other runtime features;
- redesign or rename existing public APIs;
- change operation/schema/runtime semantics;
- bump `Version` or publish a release solely for adding CI protection;
- promise behavioral compatibility beyond what tests and review establish;
- make examples or internal packages part of the documented public facade;
- suppress compatibility checks for any public API element other than the intentional `Version` value change.

## Definition of Done

This P1 item is complete when:

1. a pinned public-API compatibility helper runs in dedicated PR CI against the latest stable v2 release;
2. the helper normalizes only the `Version` value in a temporary source copy and fails closed on declaration-shape changes;
3. RED/green characterization proves it rejects a representative incompatible API change;
4. a compatible additive change is accepted by PR-mode checking;
5. release-number-only `Version` changes are accepted while removal/kind changes are rejected;
6. the release workflow validates proposed SemVer against the same stable API baseline before publication;
7. release characterization proves patch/minor/incompatible cases behave as designed;
8. black-box `package oashttp_test` consumer contracts compile/run across Go 1.22-1.26;
9. existing formatting, zero-runtime-dependency, module, vet, repeated test, race, coverage, OpenAPI golden, fuzz, benchmark, and vulnerability gates remain green;
10. no runtime/public API behavior is changed by the final diff;
11. `CONTRIBUTING.md` and the architecture note document the enforced compatibility policy;
12. the implementation is reviewed and merged through the normal PR flow.

Because this item adds only compatibility infrastructure and tests, it does not require a new `oashttp` release by itself. A later source release will automatically be subject to the new release-time SemVer gate.
