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
- Produces: `scripts/check-public-api.sh [vX.Y.Z]`; no argument = PR/suggestion mode, one canonical release argument = release SemVer mode. Exit 0 means compatible/valid; any ambiguity or incompatibility is non-zero.

- [ ] **Step 1: Create the implementation branch from the approved design/plan head**

Create `fix/public-api-compatibility-gate` from the final `design/public-api-compatibility-gate` commit so the spec and plan travel with the implementation PR.

- [ ] **Step 2: Add the helper with fail-closed baseline and Version normalization**

Use this structure:

```bash
#!/usr/bin/env bash
set -euo pipefail

GORELEASE='golang.org/x/exp/cmd/gorelease@v0.0.0-20260727155853-b88d891fe743'
proposed="${1:-}"

if [ "$#" -gt 1 ]; then
  echo "usage: $0 [vX.Y.Z]" >&2
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

baseline_version="$(git show "${baseline}:version.go" | extract_version)"
current_version="$(extract_version < version.go)"

if [ -z "$baseline_version" ] || [ -z "$current_version" ]; then
  echo 'version.go must contain exactly: const Version = "X.Y.Z"' >&2
  exit 1
fi
if [ "$(grep -Ec '^const Version = "[0-9]+\.[0-9]+\.[0-9]+"$' version.go)" -ne 1 ]; then
  echo 'current Version declaration is ambiguous' >&2
  exit 1
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

git archive HEAD | tar -x -C "$work"
sed -i "s/^const Version = \"${current_version}\"$/const Version = \"${baseline_version}\"/" "$work/version.go"
if [ "$(extract_version < "$work/version.go")" != "$baseline_version" ]; then
  echo 'failed to normalize Version in temporary source tree' >&2
  exit 1
fi

args=("-base=${baseline}")
if [ -n "$proposed" ]; then
  args+=("-version=${proposed}")
fi

(
  cd "$work"
  GOTOOLCHAIN=local go run "$GORELEASE" "${args[@]}"
)
```

Before committing, tighten the implementation so baseline and current `version.go` each contain exactly one supported declaration; reject malformed proposed values early or let `gorelease` reject them canonically. Do not modify the live checkout during normalization.

- [ ] **Step 3: Run the helper on the API-identical branch**

Run in CI/Actions with Go 1.26:

```bash
bash scripts/check-public-api.sh
```

Expected: PASS against `v2.0.2`; output should report no incompatible public API change and suggest a patch-level next version when no compatible addition exists.

- [ ] **Step 4: RED-characterize an incompatible public API change**

Temporarily change the public middleware type from:

```go
type Middleware func(http.Handler) http.Handler
```

to:

```go
type Middleware func(http.Handler) (http.Handler, error)
```

and adjust only enough temporary code to keep package loading valid if required. Run only the compatibility check needed to observe the API diagnostic:

```bash
bash scripts/check-public-api.sh
```

Expected: FAIL with an incompatible change involving `Middleware`. Record the failing CI/run evidence, then fully restore `middleware.go` and any temporary support edits before continuing.

- [ ] **Step 5: Characterize a compatible additive API change**

Temporarily add a standalone root file:

```go
package oashttp

// CompatibilityProbe exists only for compatibility-gate characterization.
func CompatibilityProbe() {}
```

Run:

```bash
bash scripts/check-public-api.sh
```

Expected: PASS while reporting a compatible addition / minor-version suggestion. Record evidence, then delete the temporary file completely.

- [ ] **Step 6: Characterize the Version exception**

Temporarily change only:

```go
const Version = "2.0.2"
```

to:

```go
const Version = "2.0.3"
```

Run `bash scripts/check-public-api.sh`; expected PASS because only the temporary comparison copy is normalized. Then temporarily change the declaration to `var Version = "2.0.3"`; expected FAIL before `gorelease`. Restore the real `const Version = "2.0.2"` source after evidence is collected.

- [ ] **Step 7: Commit the permanent helper only**

Final diff for this task must contain `scripts/check-public-api.sh` plus spec/plan only; no characterization API mutation remains.

---

### Task 2: External consumer contract tests

**Files:**
- Create: `public_api_contract_test.go`

**Interfaces:**
- Consumes: exported `oashttp` facade only by declaring `package oashttp_test` and importing `oashttp "github.com/sevlumen/oashttp/v2"`.
- Produces: compile/run contracts for application construction, middleware/groups, typed/raw registration, result helpers, security aliases/providers, docs endpoints, operation metadata, `Version`, and `Build`/`MustBuild`.

- [ ] **Step 1: Add external-package test fixtures**

Use consumer-owned types such as:

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

- [ ] **Step 2: Add one high-value downstream integration test**

Build an app using only exported APIs:

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
    if err := app.Use(middleware); err != nil { t.Fatal(err) }

    group := app.Group("/api").Group("/users")
    if err := group.Use(middleware); err != nil { t.Fatal(err) }

    oashttp.MapGet(group, "/{id:uuid}", func(ctx context.Context, in contractInput) oashttp.Result[contractOutput] {
        if oashttp.OperationID(ctx) != "getContractUser" { t.Fatalf("operation id=%q", oashttp.OperationID(ctx)) }
        if info, ok := oashttp.OperationFromContext(ctx); !ok || info.Method != http.MethodGet { t.Fatalf("operation info=%+v ok=%v", info, ok) }
        return oashttp.OK(contractOutput{ID: in.ID}).WithHeader("X-Result", "1")
    }).WithOperationID("getContractUser").WithValidation().WithTags("Contract").WithSummary("Get contract user").WithDescription("Consumer contract").Use(middleware).RequireSecurity("apiKey").Produces(http.StatusOK).ProducesProblem(http.StatusUnauthorized)

    oashttp.MapHandler(group, http.MethodGet, "/raw", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
        w.WriteHeader(http.StatusNoContent)
    })).WithOperationID("getRawContract").WithTags("Contract").Consumes("application/octet-stream").Produces(http.StatusNoContent)

    if err := app.MapOpenAPI("/openapi.json"); err != nil { t.Fatal(err) }
    if err := app.MapSwaggerUI("/swagger", oashttp.SwaggerUIConfig{DocumentURL: "/openapi.json"}); err != nil { t.Fatal(err) }

    handler, err := app.Build()
    if err != nil { t.Fatal(err) }
    if app.MustBuild() == nil { t.Fatal("MustBuild returned nil") }

    id := "550e8400-e29b-41d4-a716-446655440000"
    req := httptest.NewRequest(http.MethodGet, "/api/users/"+id, nil)
    req.Header.Set("X-API-Key", "consumer")
    rec := httptest.NewRecorder()
    handler.ServeHTTP(rec, req)
    if rec.Code != http.StatusOK { t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String()) }
    if !strings.Contains(rec.Body.String(), id) { t.Fatalf("body=%s", rec.Body.String()) }

    docs := httptest.NewRecorder()
    handler.ServeHTTP(docs, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
    if docs.Code != http.StatusOK { t.Fatalf("openapi status=%d", docs.Code) }
}
```

If the exact security middleware requires authentication semantics that make the representative request unsuitable, keep the compile contract for `SecurityProvider`/`RequireSecurity` in one app and use a separate unsecured typed route for the behavioral request. Do not access unexported state.

- [ ] **Step 3: Run consumer contracts across the normal suite**

Run:

```bash
go test ./... -run TestPublicFacadeConsumerContract -count=1
```

Expected: PASS. The normal CI matrix will subsequently compile/run the test under Go 1.22–1.26.

- [ ] **Step 4: Commit external consumer contracts**

Commit only the external-package test file for this task.

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

Push the permanent helper + consumer tests + CI job and require the dedicated job plus all existing jobs to pass. Inspect the compatibility log to confirm the baseline selected is `v2.0.2` and the normalized `Version` exception does not hide any other API diagnostic.

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

Do not alter the existing branch/main/tag identity protections.

- [ ] **Step 2: Characterize release SemVer without publishing**

Use controlled temporary branch states and run the helper directly rather than creating a `publish/v*` branch:

```bash
bash scripts/check-public-api.sh v2.0.3
```

with unchanged API except normalized Version: expected PASS.

Add temporary `CompatibilityProbe`; run with `v2.0.3`: expected FAIL because compatible addition requires a minor release. Run the same temporary addition with `v2.1.0`: expected PASS. Apply the temporary incompatible `Middleware` mutation and run with `v2.1.0`: expected FAIL. Restore every mutation afterward.

- [ ] **Step 3: Commit release gate**

Commit `.github/workflows/release.yml` only after characterization evidence matches the intended SemVer matrix.

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

Add to `CONTRIBUTING.md` under Compatibility:

```markdown
CI compares the current public API with the latest stable v2 release. Backward-incompatible API changes cannot be merged into v2 and require a new major module path. Backward-compatible exported API additions require a minor release; patch releases must not add exported API. The exported `Version` constant is the only value-normalization exception: its release string may change, but the symbol must remain an exported constant with the same public shape.

The guarded release workflow reruns the public-API comparison with the proposed release tag and rejects SemVer mismatches before creating a tag or GitHub Release.
```

Add `bash scripts/check-public-api.sh` to local quality-gate guidance, noting that tags/network access are required.

- [ ] **Step 2: Document the architecture enforcement**

Add one concise rule/note to `docs/architecture.md`: root facade source compatibility within v2 is machine-enforced against the latest stable v2 release by PR CI and release SemVer gates; behavioral compatibility still relies on tests/review.

- [ ] **Step 3: Run final branch verification on the exact final head**

Require a fresh full CI run on the final branch head: Go 1.22–1.26, formatting, zero runtime dependencies, module consistency, vet, repeated tests, race, coverage, golden OpenAPI, route fuzz, binding fuzz, benchmarks, vulnerability scan, and the new public API compatibility job.

Also confirm:

```bash
git diff --exit-code v2.0.2 -- go.mod go.sum version.go
```

Expected: no runtime dependency or version change attributable to this P1 work.

- [ ] **Step 4: Open one PR against `main` and review the actual diff**

PR scope must contain only: approved spec/plan, helper, external-package tests, CI/release workflow gates, and contributor/architecture docs. No characterization mutation may remain.

- [ ] **Step 5: Require PR-triggered CI and review gates**

Require all PR jobs to pass; inspect PR diff, comments, submitted reviews, review threads, mergeability, and exact unchanged head SHA.

- [ ] **Step 6: Squash-merge only after review is clean**

Use the expected final head SHA when merging. Do not create a release for this infrastructure-only P1 item.

- [ ] **Step 7: Require post-merge `main` CI**

Verify the push-triggered CI on the exact squash commit completes successfully, including the new compatibility job against `v2.0.2`.

## Definition of Done

P1 is complete only when the compatibility helper has been characterized against incompatible/additive/Version cases, consumer contracts run externally across the Go matrix, PR and release workflows enforce the intended rules, final PR review is clean, the change is merged to `main`, and post-merge CI is green. This item does not publish a new release by itself.
