# Public API Compatibility Gate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Machine-enforce the released v2 public API compatibility promise in PR and release CI, while adding external-package consumer contracts and preserving zero runtime dependencies.

**Architecture:** A single `scripts/check-public-api.sh` helper resolves the latest stable v2 tag, validates/normalizes only the release value of `oashttp.Version` in a temporary source archive, and invokes a pinned `gorelease`. PR CI calls it without a proposed version; the guarded release workflow calls it with the proposed tag so SemVer is verified before publication. Black-box `package oashttp_test` tests protect representative downstream usage shapes independently of the API-diff tool.

**Tech Stack:** Go 1.22–1.26, Bash, Git, GitHub Actions, `golang.org/x/exp/cmd/gorelease@v0.0.0-20260727155853-b88d891fe743`, standard-library `net/http`/`httptest`.

## Global Constraints

- Go 1.22 remains the minimum supported library version.
- `go.mod` must retain zero third-party runtime requirements; compatibility tooling is ephemeral CI tooling only.
- No runtime/public API behavior changes and no `Version` bump for this infrastructure-only item.
- The latest stable v2 tag is selected dynamically; prereleases are excluded.
- Only the string value of exported `const Version` may be normalized for comparison; its identifier, constant kind, declaration shape, and public availability remain protected.
- PR checking rejects incompatible API changes while allowing compatible additions.
- Release checking rejects incompatible changes and enforces patch/minor SemVer against the same stable baseline.
- Temporary characterization mutations must not remain in the final PR diff.

---

### Task 1: Public API compatibility helper

**Files:**
- Create: `scripts/check-public-api.sh`
- Test/characterize: temporary commits on `fix/public-api-compatibility-gate`; no permanent production mutation

**Interfaces:**
- Consumes: Git repository with stable v2 tags and root `version.go` containing exactly one line `const Version = "X.Y.Z"`.
- Produces: `scripts/check-public-api.sh [vX.Y.Z]`; no argument = PR/suggestion mode, one canonical stable-v2 release argument = release SemVer mode. Exit 0 means compatible/valid; any ambiguity or incompatibility is non-zero.

- [ ] **Step 1: Create the implementation branch from the approved design/plan head**

Create `fix/public-api-compatibility-gate` from the final `design/public-api-compatibility-gate` commit so the spec and plan travel with the implementation PR.

- [ ] **Step 2: Add the helper with fail-closed baseline and Version normalization**

Implement `scripts/check-public-api.sh` with this concrete behavior:

```bash
#!/usr/bin/env bash
set -euo pipefail

GORELEASE='golang.org/x/exp/cmd/gorelease@v0.0.0-20260727155853-b88d891fe743'
proposed="${1:-}"

if [ "$#" -gt 1 ]; then
  echo "usage: $0 [vX.Y.Z]" >&2
  exit 2
fi
if [ -n "$proposed" ] && ! [[ "$proposed" =~ ^v2\.[0-9]+\.[0-9]+$ ]]; then
  echo "proposed release must match v2.X.Y" >&2
  exit 2
fi

mapfile -t stable_tags < <(
  git tag --list 'v2.*' |
    grep -E '^v2\.[0-9]+\.[0-9]+$' |
    sort -V
)

baseline=''
for tag in "${stable_tags[@]}"; do
  if [ -n "$proposed" ]; then
    first="$(printf '%s\n%s\n' "$tag" "$proposed" | sort -V | head -n1)"
    if [ "$tag" = "$proposed" ] || [ "$first" != "$tag" ]; then
      continue
    fi
  fi
  baseline="$tag"
done

if [ -z "$baseline" ]; then
  echo "no stable v2 compatibility baseline found" >&2
  exit 1
fi

extract_version() {
  sed -n 's/^const Version = "\([0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*\)"$/\1/p'
}

baseline_source="$(git show "${baseline}:version.go")"
baseline_count="$(printf '%s\n' "$baseline_source" | grep -Ec '^const Version = "[0-9]+\.[0-9]+\.[0-9]+"$')"
current_count="$(grep -Ec '^const Version = "[0-9]+\.[0-9]+\.[0-9]+"$' version.go)"
if [ "$baseline_count" -ne 1 ] || [ "$current_count" -ne 1 ]; then
  echo 'version.go must contain exactly one const Version = "X.Y.Z" declaration' >&2
  exit 1
fi

baseline_version="$(printf '%s\n' "$baseline_source" | extract_version)"
current_version="$(extract_version < version.go)"
if [ -z "$baseline_version" ] || [ -z "$current_version" ]; then
  echo 'failed to read Version declaration' >&2
  exit 1
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

git archive HEAD | tar -x -C "$work"
sed -i "s/^const Version = \"${current_version}\"$/const Version = \"${baseline_version}\"/" "$work/version.go"
if [ "$(grep -Ec '^const Version = "[0-9]+\.[0-9]+\.[0-9]+"$' "$work/version.go")" -ne 1 ] ||
   [ "$(extract_version < "$work/version.go")" != "$baseline_version" ]; then
  echo 'failed to normalize Version in temporary source tree' >&2
  exit 1
fi

args=("-base=${baseline}")
if [ -n "$proposed" ]; then
  args+=("-version=${proposed}")
fi

echo "public API baseline: ${baseline}"
(
  cd "$work"
  GOTOOLCHAIN=local go run "$GORELEASE" "${args[@]}"
)
```

Do not modify the live checkout during normalization. Tool/network/package-load failures propagate as non-zero exits.

- [ ] **Step 3: Run the helper on the API-identical branch**

Run in CI/Actions with Go 1.26:

```bash
bash scripts/check-public-api.sh
```

Expected: PASS against `v2.0.2`; output includes `public API baseline: v2.0.2` and no incompatible API change.

- [ ] **Step 4: RED-characterize an incompatible public API change**

Temporarily change only the exported helper signature in `operation_context.go` from:

```go
func OperationID(ctx context.Context) string {
    info, _ := OperationFromContext(ctx)
    return info.ID
}
```

to:

```go
func OperationID(ctx context.Context) (string, bool) {
    info, ok := OperationFromContext(ctx)
    return info.ID, ok
}
```

No production package calls `OperationID`, so the library package remains loadable; test packages may fail separately, which does not invalidate the compatibility-job evidence. Run:

```bash
bash scripts/check-public-api.sh
```

Expected: FAIL with an incompatible `OperationID` signature diagnostic from the API comparison. Record the CI/run evidence, then restore `operation_context.go` exactly before continuing.

- [ ] **Step 5: Characterize a compatible additive API change**

Temporarily add `compatibility_probe.go`:

```go
package oashttp

// CompatibilityProbe exists only for compatibility-gate characterization.
func CompatibilityProbe() {}
```

Run `bash scripts/check-public-api.sh`. Expected: PASS and output reports the compatible addition / suggests a minor release. Record evidence, then delete `compatibility_probe.go` completely.

- [ ] **Step 6: Characterize the Version exception**

Temporarily change only `const Version = "2.0.2"` to `const Version = "2.0.3"`; `bash scripts/check-public-api.sh` must PASS because the comparison archive normalizes the release string. Then change the declaration to `var Version = "2.0.3"`; the helper must FAIL before API comparison because the required constant declaration is absent. Restore the real `const Version = "2.0.2"` source after evidence is collected.

- [ ] **Step 7: Commit the permanent helper only**

Final diff for this task contains `scripts/check-public-api.sh` plus approved spec/plan only; no characterization API mutation remains.

---

### Task 2: External consumer contract tests

**Files:**
- Create: `public_api_contract_test.go`

**Interfaces:**
- Consumes: exported `oashttp` facade only by declaring `package oashttp_test` and importing `oashttp "github.com/sevlumen/oashttp/v2"`.
- Produces: compile/run contracts for application construction, middleware/groups, typed/raw registration, result helpers, security aliases/providers, docs endpoints, operation metadata, `Version`, and `Build`/`MustBuild`.

- [ ] **Step 1: Add external-package test fixtures**

```go
package oashttp_test

import (
    "context"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"

    oashttp "github.com/sevlumen/oashttp/v2"
)

type contractInput struct {
    ID oashttp.UUID `path:"id"`
}

type contractOutput struct {
    ID oashttp.UUID `json:"id"`
}

type apiKeyProvider struct{}

func (apiKeyProvider) SecurityScheme() oashttp.SecurityScheme {
    return oashttp.SecurityScheme{Type: "apiKey", Name: "X-API-Key", In: "header"}
}

func (apiKeyProvider) Authenticate(_ context.Context, r *http.Request) (*oashttp.Principal, error) {
    return &oashttp.Principal{Subject: r.Header.Get("X-API-Key")}, nil
}
```

- [ ] **Step 2: Add one deterministic downstream integration test**

Use one app, but keep the route used for runtime assertions unsecured; a separate route carries the security compile/OpenAPI contract:

```go
func TestPublicFacadeConsumerContract(t *testing.T) {
    if oashttp.Version == "" {
        t.Fatal("Version must remain exported")
    }

    app := oashttp.New(oashttp.Config{
        Info: oashttp.Info{Title: "Contract API", Version: "1.0.0"},
        Servers: []oashttp.Server{{URL: "https://api.example.test"}},
        SecurityProviders: map[string]oashttp.SecurityProvider{"apiKey": apiKeyProvider{}},
    })

    middleware := oashttp.Middleware(func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.Header().Set("X-Contract", "1")
            next.ServeHTTP(w, r)
        })
    })
    if err := app.Use(middleware); err != nil {
        t.Fatal(err)
    }

    group := app.Group("/api").Group("/users")
    if err := group.Use(middleware); err != nil {
        t.Fatal(err)
    }

    oashttp.MapGet(group, "/{id:uuid}", func(ctx context.Context, in contractInput) oashttp.Result[contractOutput] {
        if oashttp.OperationID(ctx) != "getContractUser" {
            t.Fatalf("operation id=%q", oashttp.OperationID(ctx))
        }
        if oashttp.RoutePattern(ctx) == "" {
            t.Fatal("route pattern must be visible")
        }
        info, ok := oashttp.OperationFromContext(ctx)
        if !ok || info.Method != http.MethodGet {
            t.Fatalf("operation info=%+v ok=%v", info, ok)
        }
        return oashttp.OK(contractOutput{ID: in.ID}).WithHeader("X-Result", "1")
    }).WithOperationID("getContractUser").WithValidation().WithTags("Contract").WithSummary("Get contract user").WithDescription("Consumer contract").Use(middleware).Produces(http.StatusOK).ProducesProblem(http.StatusBadRequest)

    oashttp.MapGet(group, "/secured/{id:uuid}", func(_ context.Context, in contractInput) oashttp.Result[contractOutput] {
        return oashttp.Accepted(contractOutput{ID: in.ID})
    }).WithOperationID("getSecuredContractUser").RequireSecurity("apiKey").Produces(http.StatusAccepted).ProducesProblem(http.StatusUnauthorized)

    oashttp.MapHandler(group, http.MethodPost, "/raw", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
        w.WriteHeader(http.StatusNoContent)
    })).WithOperationID("postRawContract").WithTags("Contract").Consumes("application/octet-stream").Produces(http.StatusNoContent)

    if err := app.MapOpenAPI("/openapi.json"); err != nil {
        t.Fatal(err)
    }
    if err := app.MapSwaggerUI("/swagger", oashttp.SwaggerUIConfig{DocumentURL: "/openapi.json"}); err != nil {
        t.Fatal(err)
    }

    handler, err := app.Build()
    if err != nil {
        t.Fatal(err)
    }
    if app.MustBuild() == nil {
        t.Fatal("MustBuild returned nil")
    }

    id := "550e8400-e29b-41d4-a716-446655440000"
    rec := httptest.NewRecorder()
    handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/users/"+id, nil))
    if rec.Code != http.StatusOK {
        t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
    }
    if !strings.Contains(rec.Body.String(), id) {
        t.Fatalf("body=%s", rec.Body.String())
    }
    if rec.Header().Get("X-Contract") != "1" || rec.Header().Get("X-Result") != "1" {
        t.Fatalf("headers=%v", rec.Header())
    }

    docs := httptest.NewRecorder()
    handler.ServeHTTP(docs, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
    if docs.Code != http.StatusOK {
        t.Fatalf("openapi status=%d", docs.Code)
    }
}
```

This test must not access unexported state or rely on provider authentication for the behavioral request.

- [ ] **Step 3: Run consumer contracts across the normal suite**

Run:

```bash
go test ./... -run TestPublicFacadeConsumerContract -count=1
```

Expected: PASS. The normal CI matrix subsequently compiles/runs the test under Go 1.22–1.26.

- [ ] **Step 4: Commit external consumer contracts**

Commit only `public_api_contract_test.go` for this task.

---

### Task 3: PR CI compatibility gate

**Files:**
- Modify: `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: `scripts/check-public-api.sh`.
- Produces: dedicated `Public API compatibility` CI job using Go 1.26 and full tag history.

- [ ] **Step 1: Add a dedicated CI job**

Append:

```yaml
  api-compatibility:
    name: Public API compatibility
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v7
        with:
          go-version: "1.26.x"
          cache: false
      - name: Check public API compatibility
        shell: bash
        run: bash scripts/check-public-api.sh
```

Keep existing test/quality jobs unchanged.

- [ ] **Step 2: Verify branch CI**

Require the dedicated job plus all existing jobs to pass. Inspect the compatibility log to confirm `public API baseline: v2.0.2` and no hidden API diagnostic.

- [ ] **Step 3: Commit CI integration**

Commit `.github/workflows/ci.yml` separately from release workflow changes.

---

### Task 4: Release-time SemVer enforcement

**Files:**
- Modify: `.github/workflows/release.yml`
- Characterize: temporary branch states only; final release workflow is permanent

**Interfaces:**
- Consumes: `RELEASE_TAG` from the existing release identity step and `scripts/check-public-api.sh`.
- Produces: fail-closed SemVer gate immediately before GitHub Release publication.

- [ ] **Step 1: Insert the SemVer check before publication**

After vulnerability scanning and before `Publish GitHub Release`, add:

```yaml
      - name: Verify public API release compatibility
        shell: bash
        run: bash scripts/check-public-api.sh "$RELEASE_TAG"
```

Do not alter existing branch/main/tag identity protections.

- [ ] **Step 2: Characterize release SemVer without publishing**

With unchanged API, run:

```bash
bash scripts/check-public-api.sh v2.0.3
```

Expected: PASS.

With temporary `CompatibilityProbe`, run `bash scripts/check-public-api.sh v2.0.3`: expected FAIL because an additive exported API needs a minor release. With the same temporary addition, run `bash scripts/check-public-api.sh v2.1.0`: expected PASS. With the temporary incompatible `OperationID` signature, run `bash scripts/check-public-api.sh v2.1.0`: expected FAIL. Restore every mutation afterward.

- [ ] **Step 3: Commit release gate**

Commit `.github/workflows/release.yml` only after the SemVer matrix matches the expected results.

---

### Task 5: Documentation and final integration

**Files:**
- Modify: `CONTRIBUTING.md`
- Modify: `docs/architecture.md`
- Update: `docs/superpowers/plans/2026-08-12-public-api-compatibility-gate.md` execution evidence/checkmarks

**Interfaces:**
- Consumes: implemented helper/CI/release behavior.
- Produces: documented compatibility policy and auditable execution record.

- [ ] **Step 1: Document contributor compatibility policy**

Add under Compatibility:

```markdown
CI compares the current public API with the latest stable v2 release. Backward-incompatible API changes cannot be merged into v2 and require a new major module path. Backward-compatible exported API additions require a minor release; patch releases must not add exported API. The exported `Version` constant is the only value-normalization exception: its release string may change, but the symbol must remain an exported constant with the same public shape.

The guarded release workflow reruns the public-API comparison with the proposed release tag and rejects SemVer mismatches before creating a tag or GitHub Release.
```

Add `bash scripts/check-public-api.sh` to local quality-gate guidance and state that stable release tags plus network access for the pinned Go tool are required.

- [ ] **Step 2: Document architecture enforcement**

Add one concise rule to `docs/architecture.md`: root facade source compatibility within v2 is machine-enforced against the latest stable v2 release by PR CI and release SemVer gates; behavioral compatibility still depends on tests/review.

- [ ] **Step 3: Run final branch verification on the exact final head**

Require a fresh full CI run on the final branch head: Go 1.22–1.26, formatting, zero runtime dependencies, module consistency, vet, repeated tests, race, coverage, OpenAPI golden, route fuzz, binding fuzz, benchmarks, vulnerability scan, and the new public API compatibility job.

Confirm the branch does not change module/release metadata:

```bash
git diff --exit-code v2.0.2 -- go.mod go.sum version.go
```

Expected: no diff.

- [ ] **Step 4: Open one PR against `main` and review the actual diff**

PR scope must contain only approved spec/plan, helper, external-package tests, CI/release workflow gates, and contributor/architecture docs. No characterization mutation may remain.

- [ ] **Step 5: Require PR-triggered CI and review gates**

Require all PR jobs to pass; inspect PR diff, comments, submitted reviews, review threads, mergeability, and exact unchanged head SHA.

- [ ] **Step 6: Squash-merge only after review is clean**

Use the expected final head SHA when merging. Do not create a release for this infrastructure-only P1 item.

- [ ] **Step 7: Require post-merge `main` CI**

Verify push-triggered CI on the exact squash commit completes successfully, including the new compatibility job against the latest stable v2 tag.

## Definition of Done

P1 is complete only when the compatibility helper has been characterized against incompatible/additive/Version cases, consumer contracts run externally across the Go matrix, PR and release workflows enforce the intended rules, final PR review is clean, the change is merged to `main`, and post-merge CI is green. This item does not publish a new release by itself.
