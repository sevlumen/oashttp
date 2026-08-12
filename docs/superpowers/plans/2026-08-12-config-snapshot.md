# Config Snapshot Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `New(Config)` detach application-owned configuration containers from caller-owned `Servers` and `SecurityProviders` storage.

**Architecture:** Keep the ownership boundary in `normalizeConfig`. Clone only the mutable containers exposed by `Config`; preserve interface values and all existing defaults. `Build()` remains unchanged and consumes the already-owned `runtimeConfig` snapshot.

**Tech Stack:** Go 1.22+, standard library only, `net/http`, existing OpenAPI 3.1 model and GitHub Actions quality gates.

## Global Constraints

- No public API changes.
- No version bump.
- No new runtime dependency.
- Preserve nil container inputs as nil.
- Clone `Servers []Server` by value.
- Clone `SecurityProviders map[string]SecurityProvider` by key/interface value only; do not deep-clone provider implementations.
- Do not change `Build()` freeze semantics.
- Do not make user-supplied provider implementations responsible for new framework synchronization.
- Existing Go 1.22–1.26, race, coverage, golden, fuzz, benchmark, and `govulncheck` gates must remain unchanged.

---

### Task 1: Lock config ownership with regression tests

**Files:**
- Create: `config_snapshot_test.go`
- Read: `config.go`
- Read: `app.go`

**Interfaces:**
- Consumes: `func New(Config) *App`, `func (a *App) Build() (http.Handler, error)`, `type Config`, `type Server`, `type SecurityProvider`.
- Produces: regression coverage proving caller-owned container mutation after `New()` cannot mutate `app.config`.

- [ ] **Step 1: Add a minimal test provider and server snapshot test**

Create `config_snapshot_test.go` in package `oashttp` so the test can inspect the internal `runtimeConfig` directly without introducing production-only hooks:

```go
package oashttp

import (
    "context"
    "net/http"
    "testing"
)

type snapshotSecurityProvider struct {
    scheme SecurityScheme
}

func (p *snapshotSecurityProvider) SecurityScheme() SecurityScheme { return p.scheme }

func (p *snapshotSecurityProvider) Authenticate(context.Context, *http.Request) (*Principal, error) {
    return &Principal{}, nil
}

func TestNewSnapshotsServers(t *testing.T) {
    servers := []Server{{URL: "https://original.example", Description: "original"}}
    app := New(Config{Servers: servers})

    servers[0].URL = "https://changed.example"
    servers[0].Description = "changed"

    if got := app.config.Servers[0]; got.URL != "https://original.example" || got.Description != "original" {
        t.Fatalf("app server = %#v, want original snapshot", got)
    }
}
```

- [ ] **Step 2: Add provider map replacement/add/delete tests**

Append:

```go
func TestNewSnapshotsSecurityProviders(t *testing.T) {
    providerA := &snapshotSecurityProvider{scheme: SecurityScheme{Type: "apiKey", Name: "X-API-Key", In: "header"}}
    providerB := &snapshotSecurityProvider{scheme: SecurityScheme{Type: "apiKey", Name: "X-Other-Key", In: "header"}}

    providers := map[string]SecurityProvider{"apiKey": providerA}
    app := New(Config{SecurityProviders: providers})

    providers["apiKey"] = providerB
    providers["late"] = providerB
    delete(providers, "apiKey")

    if got := app.config.SecurityProviders["apiKey"]; got != providerA {
        t.Fatalf("apiKey provider = %T %p, want original provider %p", got, got, providerA)
    }
    if _, ok := app.config.SecurityProviders["late"]; ok {
        t.Fatal("provider added after New() leaked into app config")
    }
}
```

- [ ] **Step 3: Add nil-preservation tests**

Append:

```go
func TestNewPreservesNilConfigContainers(t *testing.T) {
    app := New(Config{})

    if app.config.Servers != nil {
        t.Fatalf("Servers = %#v, want nil", app.config.Servers)
    }
    if app.config.SecurityProviders != nil {
        t.Fatalf("SecurityProviders = %#v, want nil", app.config.SecurityProviders)
    }
}
```

- [ ] **Step 4: Run focused tests and verify RED**

Run:

```bash
go test ./... -run 'TestNewSnapshotsServers|TestNewSnapshotsSecurityProviders|TestNewPreservesNilConfigContainers' -count=1
```

Expected: the first two tests fail on the current implementation because `normalizeConfig` aliases the caller's slice/map; nil-preservation may already pass.

- [ ] **Step 5: Commit the RED tests**

```bash
git add config_snapshot_test.go
git commit -m "test: lock config snapshot ownership"
```

---

### Task 2: Snapshot mutable config containers in normalization

**Files:**
- Modify: `config.go`
- Test: `config_snapshot_test.go`

**Interfaces:**
- Consumes: `func normalizeConfig(Config) runtimeConfig`.
- Produces: application-owned `Servers` and `SecurityProviders` containers after `New()`.

- [ ] **Step 1: Add focused clone helpers**

Add to `config.go`:

```go
func cloneServers(servers []Server) []Server {
    if servers == nil {
        return nil
    }
    return append([]Server(nil), servers...)
}

func cloneSecurityProviders(providers map[string]SecurityProvider) map[string]SecurityProvider {
    if providers == nil {
        return nil
    }
    cloned := make(map[string]SecurityProvider, len(providers))
    for name, provider := range providers {
        cloned[name] = provider
    }
    return cloned
}
```

- [ ] **Step 2: Apply the ownership snapshot inside `normalizeConfig`**

Before returning `runtimeConfig`, assign:

```go
cfg.Servers = cloneServers(cfg.Servers)
cfg.SecurityProviders = cloneSecurityProviders(cfg.SecurityProviders)
```

Keep all existing defaulting behavior unchanged.

- [ ] **Step 3: Run focused tests and verify GREEN**

Run:

```bash
go test ./... -run 'TestNewSnapshotsServers|TestNewSnapshotsSecurityProviders|TestNewPreservesNilConfigContainers' -count=1
```

Expected: PASS.

- [ ] **Step 4: Add a race-focused ownership test**

Append to `config_snapshot_test.go`:

```go
func TestNewDetachesConfigContainersBeforeConcurrentUse(t *testing.T) {
    providerA := &snapshotSecurityProvider{scheme: SecurityScheme{Type: "apiKey", Name: "X-API-Key", In: "header"}}
    providerB := &snapshotSecurityProvider{scheme: SecurityScheme{Type: "apiKey", Name: "X-Other-Key", In: "header"}}

    servers := []Server{{URL: "https://original.example"}}
    providers := map[string]SecurityProvider{"apiKey": providerA}
    app := New(Config{Servers: servers, SecurityProviders: providers})

    done := make(chan struct{})
    go func() {
        defer close(done)
        for i := 0; i < 1000; i++ {
            servers[0].URL = "https://changed.example"
            providers["apiKey"] = providerB
            delete(providers, "apiKey")
            providers["apiKey"] = providerA
        }
    }()

    for i := 0; i < 1000; i++ {
        if got := app.config.Servers[0].URL; got != "https://original.example" {
            t.Fatalf("app server URL = %q, want original snapshot", got)
        }
        if got := app.config.SecurityProviders["apiKey"]; got != providerA {
            t.Fatalf("app provider changed after New(): %#v", got)
        }
    }
    <-done
}
```

This intentionally exercises the caller-owned containers after `New()` without mutating provider internals.

- [ ] **Step 5: Run the race regression**

Run:

```bash
go test -race ./... -run TestNewDetachesConfigContainersBeforeConcurrentUse -count=1
```

Expected: PASS with no race report.

- [ ] **Step 6: Commit the implementation**

```bash
git add config.go config_snapshot_test.go
git commit -m "fix: snapshot mutable config containers"
```

---

### Task 3: Verify compatibility and publish the reviewed change

**Files:**
- Verify: `.github/workflows/ci.yml`
- Verify: `go.mod`
- Verify: `testdata/users.openapi.golden.json`
- Review: `docs/superpowers/specs/2026-08-12-config-snapshot-design.md`

**Interfaces:**
- Consumes: final branch from Tasks 1–2.
- Produces: a merge-ready PR with no public API, dependency, or OpenAPI output changes.

- [ ] **Step 1: Run local-equivalent repository checks where available**

```bash
test -z "$(gofmt -l .)"
go vet ./...
go test ./... -shuffle=on -count=3
go test -race ./...
```

- [ ] **Step 2: Open a PR into `main`**

Use title:

```text
fix: snapshot mutable config containers
```

PR body must state:

- `New()` now owns copies of `Servers` and `SecurityProviders` containers;
- provider implementations are not deep-cloned;
- no public API/version/runtime dependency changes;
- caller mutation after `New()` no longer changes app configuration;
- configuration race boundary is covered by `go test -race`.

- [ ] **Step 3: Require the existing GitHub Actions matrix to pass unchanged**

Verify:

```text
Go 1.22.x   success
Go 1.23.x   success
Go 1.24.x   success
Go 1.25.x   success
Go 1.26.x   success
race         success
coverage     >= 70%
OpenAPI      golden unchanged
route fuzz   success
binding fuzz success
benchmark    success
govulncheck  no known vulnerabilities
```

- [ ] **Step 4: Review final diff and merge only if clean**

Acceptance criteria:

```text
Public API changes                 none
Runtime dependencies               zero
Config container aliasing          removed
Nil container semantics            preserved
Provider object identity            preserved
Build freeze behavior              unchanged
OpenAPI golden output              unchanged
Post-change CI                     green
```
