# Config Ownership Snapshot Design

## Status

Approved design direction; implementation not started.

## Problem

`New(config Config)` currently stores `normalizeConfig(config)` in `App`. Because `Config.Servers` is a slice and `Config.SecurityProviders` is a map, copying the `Config` value does not detach their backing storage from the caller.

A caller can therefore mutate those containers after `New()` and unintentionally alter the application's configuration before or while `Build()` reads it. This weakens the intended configuration ownership and freeze boundary and can create data races when caller-owned mutation overlaps application build or request setup.

## Goal

Make `New()` establish an application-owned snapshot of mutable configuration containers so caller mutation after construction cannot change the app's configuration or race with `Build()` through those containers.

## Non-goals

- No public API changes.
- No version bump.
- No new runtime dependency.
- No deep cloning of `SecurityProvider`, `Authenticator`, `Authorizer`, `Validator`, `FailureFormatter`, middleware, or other interface/function implementations.
- No change to the existing `Build()` freeze semantics.
- No attempt to make arbitrary user-supplied provider implementations internally thread-safe.

## Design

### Ownership boundary

`New(config Config)` remains the public construction boundary. During normalization, oashttp copies the mutable container fields that are currently exposed by `Config`:

- `Servers []Server`
- `SecurityProviders map[string]SecurityProvider`

After `New()` returns, the `App` owns those copied containers.

The copy is shallow by design:

- `Server` values are copied by value.
- `SecurityProviders` keys and interface values are copied into a new map.
- The concrete provider objects referenced by interface values are not cloned.

This means replacing, adding, or deleting entries in the caller's original map after `New()` cannot affect the app, but mutating the internal state of a provider object that both caller and app reference remains the provider owner's responsibility.

### Nil semantics

Normalization preserves `nil` for nil container inputs. Non-nil containers are copied into distinct storage. The distinction between nil and empty containers is not otherwise given new public semantics.

### Placement

The ownership snapshot belongs in configuration normalization rather than `Build()` because `New()` is where ownership transfers from caller-provided configuration to the application.

Conceptually:

```text
caller Config
    |
    v
   New()
    |
    +-- normalize defaults
    +-- clone Servers container
    +-- clone SecurityProviders container
    |
    v
App-owned runtimeConfig
    |
    v
  Build()
```

`Build()` continues to snapshot application registration state under `App.mu`; it should not need to compensate for caller-owned config aliases.

## Behavioral contract

Given:

```go
cfg := Config{
    Servers: []Server{{URL: "https://original.example"}},
    SecurityProviders: map[string]SecurityProvider{
        "apiKey": providerA,
    },
}

app := New(cfg)
```

Subsequent caller mutations such as:

```go
cfg.Servers[0].URL = "https://changed.example"
cfg.SecurityProviders["apiKey"] = providerB
cfg.SecurityProviders["other"] = providerC
delete(cfg.SecurityProviders, "apiKey")
```

must not change the configuration observed by `app`.

The app must continue to observe the server value and provider mapping captured when `New()` was called.

## Concurrency contract

After `New()` returns, concurrent mutation of the caller's original `Servers` slice elements or `SecurityProviders` map must not race with `App.Build()` through shared container storage.

This contract does not cover mutation of concrete objects stored behind interface values. For example, if `providerA` contains mutable fields and the caller changes them concurrently with authentication, the provider implementation must provide its own synchronization.

## Testing

Add focused regression tests around construction and build behavior.

### Server snapshot

Create a config with one server, call `New()`, mutate the caller's original slice element, build the app, and assert the generated OpenAPI document still contains the original server value.

### Security provider snapshot

Create a config with a named security provider, call `New()`, then replace/delete entries in the caller's original map. Register an operation requiring the original provider and assert `Build()` still succeeds and emits the original security scheme.

A complementary case should prove that adding a provider to the caller's map after `New()` does not make that provider available to the app.

### Race regression

Add a test suitable for `go test -race` that calls `New()` first, then mutates the caller's original containers concurrently with `Build()`. The test must avoid mutating shared provider internals; it is specifically intended to detect container aliasing.

## Compatibility

The change is behavior-preserving for callers that already treat `Config` as construction input. It intentionally changes only the accidental aliasing behavior where callers mutate the original slice or map after `New()` and expect those mutations to affect the app.

That aliasing is not part of the documented configuration API and conflicts with the existing configuration ownership/freeze model.

## Verification

The implementation must pass the existing repository quality gates unchanged:

- Go 1.22 through Go 1.26 matrix
- formatting
- zero runtime dependency check
- module consistency
- `go vet ./...`
- repeated shuffled tests
- race detector
- coverage floor >= 70%
- OpenAPI golden verification
- route and binding fuzz targets
- benchmarks
- `govulncheck ./...`

## Follow-up

After this ownership fix is complete, a separate design can convert selected rules from `docs/architecture.md` into executable architecture contract tests. That follow-up is intentionally out of scope for this change.