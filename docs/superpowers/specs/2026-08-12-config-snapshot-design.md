# Config Ownership Snapshot Design

## Status

Implemented on `refactor/config-snapshot` and verified by GitHub Actions CI #175 on head `d7eb74964178b51029216e92578e254979e24114`.

## Problem

`New(config Config)` previously stored `normalizeConfig(config)` in `App`. Because `Config.Servers` is a slice and `Config.SecurityProviders` is a map, copying the `Config` value did not detach their backing storage from the caller.

A caller could therefore mutate those containers after `New()` and unintentionally alter the application's configuration before or while `Build()` read it. This weakened the intended configuration ownership and freeze boundary and could create data races when caller-owned mutation overlapped application build or request setup.

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

`Build()` continues to snapshot application registration state under `App.mu`; it does not compensate for caller-owned config aliases.

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

The app continues to observe the server value and provider mapping captured when `New()` was called.

## Concurrency contract

After `New()` returns, concurrent mutation of the caller's original `Servers` slice elements or `SecurityProviders` map does not race with `App.Build()` through shared container storage.

This contract does not cover mutation of concrete objects stored behind interface values. For example, if `providerA` contains mutable fields and the caller changes them concurrently with authentication, the provider implementation must provide its own synchronization.

## Testing

Focused regression coverage was added around construction and build behavior.

### Server snapshot

A config with one server is passed to `New()`, the caller's original slice element is mutated, and the app retains the original server value.

### Security provider snapshot

A config with a named security provider is passed to `New()`, then the caller replaces, adds, and deletes entries in the original map. The app retains the original provider mapping and does not observe providers added after construction.

### Race regression

A race-focused test mutates the caller's original `Servers` slice and `SecurityProviders` map concurrently with `Build()`. The operation requires the named provider so build-time code consumes both app-owned snapshots. `go test -race ./...` passes.

## Compatibility

The change is behavior-preserving for callers that already treat `Config` as construction input. It intentionally changes only the accidental aliasing behavior where callers mutate the original slice or map after `New()` and expect those mutations to affect the app.

That aliasing is not part of the documented configuration API and conflicts with the existing configuration ownership/freeze model.

## Verification

GitHub Actions CI #175 passed on implementation head `d7eb74964178b51029216e92578e254979e24114`:

- Go 1.22 through Go 1.26 matrix: pass
- formatting: pass
- zero runtime dependency check: pass
- module consistency: pass
- `go vet ./...`: pass
- repeated shuffled tests: pass
- race detector: pass
- total statement coverage: 71.6% against the 70% floor
- OpenAPI golden verification: pass with no diff
- route fuzz target: pass
- binding fuzz target: pass
- benchmarks: pass
- `govulncheck ./...`: no known vulnerabilities

## Follow-up

After this ownership fix is complete, a separate design can convert selected rules from `docs/architecture.md` into executable architecture contract tests. That follow-up is intentionally out of scope for this change.