# Schema ↔ encoding/json Conformance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make generated schemas match default `encoding/json` v1 wire semantics for embedded-field selection, dominance/conflicts, `json:",string"`, and JSON-compatible map keys, then publish `v2.0.3` if all compatibility and release gates pass.

**Architecture:** Add a focused `internal/schema/jsonfields.go` resolver that selects deterministic JSON fields before schema reflection. `structSchema` consumes only selected fields; a narrow quoted-wire transform handles `json:",string"`; map-key representability remains a small reflector helper. Runtime binding and validation are unchanged.

**Tech Stack:** Go 1.22–1.26, standard-library `encoding/json`, `encoding.TextMarshaler`, `reflect`, `strings`, `unicode`, existing `internal/schema`, existing `internal/validationrule`, GitHub Actions, existing public-API compatibility/release gates.

## Global Constraints

- Target default/legacy `encoding/json` v1 behavior supported by Go 1.22–1.26; do not implement experimental `encoding/json/v2` semantics.
- No exported `oashttp` API additions or signature changes.
- No third-party runtime dependencies; `go.mod` remains dependency-free at runtime.
- Do not change runtime binding or validation traversal.
- `validate:"required"` remains the source of OpenAPI `required`; `omitempty` and `omitzero` do not redefine it.
- General `json.Marshaler` inference, interface schema policy, and byte-slice special encoding are out of scope.
- `SchemaProvider` remains the explicit escape hatch for custom JSON representations.
- Hidden or ambiguous fields must not be reflected, schema-validated, or allowed to fail schema generation.
- Characterization and implementation follow RED → GREEN; temporary experiments must not remain in the final PR diff.
- If the final diff remains patch-compatible, release target is `v2.0.3` from a verified `main` commit through `publish/v2.0.3`.

---

## File map

- Create `internal/schema/jsonfields.go`: JSON tag metadata, v1-style breadth-first field discovery, dominance/conflict resolution, quoted applicability.
- Create `internal/schema/jsonfields_test.go`: resolver and `encoding/json` field-selection conformance matrix.
- Modify `internal/schema/reflector.go`: consume resolved fields and accept JSON-compatible map keys.
- Modify `internal/schema/tags.go`: quoted-wire transformation and enum/example lexical conversion.
- Modify `internal/schema/reflector_test.go`: schema-level `,string` and map-key contracts.
- Modify `version.go`, `version_test.go`, `README.md`, `CHANGELOG.md`: release-ready `v2.0.3` metadata only after behavior is green.
- Reuse `.github/workflows/ci.yml`, `.github/workflows/release.yml`, and `scripts/check-public-api.sh` unchanged unless verification uncovers an independent release-gate defect.

---

### Task 1: RED field-selection conformance matrix

**Files:**
- Create: `internal/schema/jsonfields_test.go`

**Interfaces:**
- Consumes: existing `Registry.Ref(reflect.Type)` and generated components.
- Produces: failing tests that define selected JSON property names using `encoding/json` itself as the observable oracle.

- [ ] **Step 1: Create implementation branch**

Create `fix/schema-json-conformance` from the final `design/schema-json-conformance` head so approved spec and plan travel with the PR.

- [ ] **Step 2: Add test helpers**

Create `internal/schema/jsonfields_test.go`:

```go
package schema

import (
    "encoding/json"
    "reflect"
    "sort"
    "testing"
)

func marshaledObjectKeys(t *testing.T, value any) []string {
    t.Helper()
    data, err := json.Marshal(value)
    if err != nil {
        t.Fatal(err)
    }
    var object map[string]json.RawMessage
    if err := json.Unmarshal(data, &object); err != nil {
        t.Fatalf("decode %s: %v", data, err)
    }
    keys := make([]string, 0, len(object))
    for key := range object {
        keys = append(keys, key)
    }
    sort.Strings(keys)
    return keys
}

func schemaObjectKeys(t *testing.T, value any) []string {
    t.Helper()
    registry := NewRegistry()
    ref, err := registry.Ref(reflect.TypeOf(value))
    if err != nil {
        t.Fatal(err)
    }
    const prefix = "#/components/schemas/"
    refText, ok := (*ref)["$ref"].(string)
    if !ok || len(refText) <= len(prefix) || refText[:len(prefix)] != prefix {
        t.Fatalf("unexpected ref %#v", *ref)
    }
    component := registry.Components()[refText[len(prefix):]]
    if component == nil {
        t.Fatalf("missing component for %q", refText)
    }
    properties := (*component)["properties"].(map[string]any)
    keys := make([]string, 0, len(properties))
    for key := range properties {
        keys = append(keys, key)
    }
    sort.Strings(keys)
    return keys
}
```

- [ ] **Step 3: Add selection fixtures**

```go
type jsonEmbedded struct {
    Promoted string `json:"promoted"`
}

type jsonEmbeddedPointer struct {
    PointerValue string `json:"pointerValue"`
}

type jsonNamedEmbedded struct {
    Value string `json:"value"`
}

type jsonLeft struct {
    Conflict string `json:"conflict"`
}

type jsonRight struct {
    Conflict string `json:"conflict"`
}

type jsonTaggedWinner struct {
    Value string `json:"winner"`
}

type jsonUntaggedLoser struct {
    Winner string
}

type jsonDeeper struct {
    Value string `json:"value"`
}

type jsonOuter struct {
    jsonDeeper
    Value string `json:"value"`
}

type jsonSelectionFixture struct {
    jsonEmbedded
    *jsonEmbeddedPointer
    jsonNamedEmbedded `json:"named"`
    Skip    string `json:"-"`
    Dash    string `json:"-,"`
    Invalid string `json:"bad\\name"`
    Spaced  string `json:"space name"`
}

type jsonAmbiguousFixture struct {
    jsonLeft
    jsonRight
}

type jsonTaggedDominanceFixture struct {
    jsonTaggedWinner
    jsonUntaggedLoser
}

type jsonOptionsOnlyLeft struct {
    Name string `json:",omitempty"`
}

type jsonOptionsOnlyRight struct {
    Name string
}

type jsonOptionsOnlyAmbiguous struct {
    jsonOptionsOnlyLeft
    jsonOptionsOnlyRight
}
```

- [ ] **Step 4: Add RED oracle test**

```go
func TestSchemaFieldSelectionMatchesEncodingJSON(t *testing.T) {
    pointer := &jsonEmbeddedPointer{PointerValue: "p"}
    cases := []any{
        jsonSelectionFixture{
            jsonEmbedded:        jsonEmbedded{Promoted: "a"},
            jsonEmbeddedPointer: pointer,
            jsonNamedEmbedded:   jsonNamedEmbedded{Value: "n"},
            Skip:                "skip",
            Dash:                "dash",
            Invalid:             "fallback",
            Spaced:              "space",
        },
        jsonAmbiguousFixture{
            jsonLeft:  jsonLeft{Conflict: "left"},
            jsonRight: jsonRight{Conflict: "right"},
        },
        jsonTaggedDominanceFixture{
            jsonTaggedWinner:  jsonTaggedWinner{Value: "tagged"},
            jsonUntaggedLoser: jsonUntaggedLoser{Winner: "plain"},
        },
        jsonOuter{jsonDeeper: jsonDeeper{Value: "deep"}, Value: "outer"},
        jsonOptionsOnlyAmbiguous{
            jsonOptionsOnlyLeft:  jsonOptionsOnlyLeft{Name: "left"},
            jsonOptionsOnlyRight: jsonOptionsOnlyRight{Name: "right"},
        },
    }
    for _, value := range cases {
        value := value
        t.Run(reflect.TypeOf(value).Name(), func(t *testing.T) {
            got := schemaObjectKeys(t, value)
            want := marshaledObjectKeys(t, value)
            if !reflect.DeepEqual(got, want) {
                t.Fatalf("schema keys=%v encoding/json keys=%v", got, want)
            }
        })
    }
}
```

This matrix locks direct embedding, anonymous `*struct`, unexported anonymous-struct traversal, explicitly named anonymous fields, outer shadowing, equal-depth ambiguity, tagged dominance, options-only tags, `json:"-"`, `json:"-,"`, invalid-name fallback, and the legacy ASCII-space tag-name case.

- [ ] **Step 5: Run RED test**

```bash
go test ./internal/schema -run TestSchemaFieldSelectionMatchesEncodingJSON -count=1
```

Expected: assertion FAIL under the current linear `structSchema`, including anonymous `*struct` and conflict/dominance drift. Compile/setup failures do not count as RED evidence.

- [ ] **Step 6: Commit test-only RED checkpoint**

Commit `internal/schema/jsonfields_test.go` only for behavior characterization and record the failing CI run before production changes.

---

### Task 2: JSON tag parser and v1-style field resolver

**Files:**
- Create: `internal/schema/jsonfields.go`
- Modify: `internal/schema/tags.go`
- Test: `internal/schema/jsonfields_test.go`

**Interfaces:**

```go
type jsonTag struct {
    Name      string
    HasName   bool
    OmitEmpty bool
    String    bool
    Skip      bool
}

type jsonField struct {
    Field  reflect.StructField
    Index  []int
    Name   string
    Tagged bool
    Quoted bool
}

func parseJSONTag(field reflect.StructField) jsonTag
func resolveJSONFields(t reflect.Type) []jsonField
func validJSONTagName(name string) bool
func quotedJSONType(t reflect.Type) bool
```

- [ ] **Step 1: Add parser RED test**

```go
func TestParseJSONTagMetadata(t *testing.T) {
    typ := reflect.TypeOf(struct {
        Plain   string
        Named   int    `json:"named,omitempty,string"`
        Options int    `json:",omitempty,string"`
        Skip    string `json:"-"`
        Dash    string `json:"-,"`
        Invalid string `json:"bad\\name"`
        Spaced  string `json:"space name"`
    }{})
    cases := []struct {
        field int
        want  jsonTag
    }{
        {0, jsonTag{}},
        {1, jsonTag{Name: "named", HasName: true, OmitEmpty: true, String: true}},
        {2, jsonTag{OmitEmpty: true, String: true}},
        {3, jsonTag{Skip: true}},
        {4, jsonTag{Name: "-", HasName: true}},
        {5, jsonTag{}},
        {6, jsonTag{Name: "space name", HasName: true}},
    }
    for _, tc := range cases {
        if got := parseJSONTag(typ.Field(tc.field)); !reflect.DeepEqual(got, tc.want) {
            t.Fatalf("field %d tag=%+v want=%+v", tc.field, got, tc.want)
        }
    }
}
```

- [ ] **Step 2: Run parser test to verify RED**

```bash
go test ./internal/schema -run TestParseJSONTagMetadata -count=1
```

Expected: compile FAIL until the new parser API exists.

- [ ] **Step 3: Implement exact legacy tag-name validity**

In `internal/schema/jsonfields.go`:

```go
func validJSONTagName(name string) bool {
    if name == "" {
        return false
    }
    const punctuation = "!#$%&()*+-./:;<=>?@[]^_{|}~ "
    for _, r := range name {
        if strings.ContainsRune(punctuation, r) || unicode.IsLetter(r) || unicode.IsDigit(r) {
            continue
        }
        return false
    }
    return true
}
```

ASCII space is intentionally accepted because legacy v1 accepts it; quote, backslash, comma, controls, and other whitespace fail.

- [ ] **Step 4: Implement tag parsing**

`parseJSONTag` inspects the raw tag first: only raw `json:"-"` means `Skip=true`; `json:"-,"` is the explicit name `-`. Split remaining tag at the first comma, validate the name, and scan comma-separated options for exact `omitempty` and `string` tokens. Invalid explicit names behave as no explicit name.

- [ ] **Step 5: Implement quoted applicability**

Mirror legacy v1 field discovery: if the field type is an unnamed pointer, dereference one pointer layer, then accept bool, string, signed integers, unsigned integers including `uintptr`, and floats.

```go
func quotedJSONType(t reflect.Type) bool {
    if t.Name() == "" && t.Kind() == reflect.Pointer {
        t = t.Elem()
    }
    switch t.Kind() {
    case reflect.Bool,
        reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
        reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
        reflect.Float32, reflect.Float64,
        reflect.String:
        return true
    default:
        return false
    }
}
```

- [ ] **Step 6: Implement breadth-first discovery with duplicate embedding counts**

Use:

```go
type jsonFieldScan struct {
    Type  reflect.Type
    Index []int
}
```

Algorithm:

1. `next` starts with the root struct.
2. At each depth, promote `next` to `current`, carry `nextCount` to `count`, and create fresh `nextCount`.
3. Skip scanning a struct type already visited at an earlier depth; shallower discovery dominates deeper copies.
4. For each field, handle anonymous visibility first: unexported anonymous struct/`*struct` may be traversed; unexported anonymous non-struct and unexported ordinary fields are ignored.
5. Parse JSON tag and skip raw `-`.
6. Build the full copied index path.
7. Dereference an anonymous pointer for embedding decisions.
8. If the field has a valid explicit JSON name, is non-anonymous, or is anonymous but not a struct, record a `jsonField` candidate with selected name, tag flag, and `Quoted=tag.String && quotedJSONType(field.Type)`.
9. If `count[parent.Type] > 1`, append one duplicate of that candidate so multiple instances of the same embedded type remain ambiguous.
10. Otherwise, for anonymous struct promotion, increment `nextCount[embeddedType]` and enqueue the type only on its first occurrence at that next depth.

Do not use ancestry-only traversal.

- [ ] **Step 7: Implement dominance and deterministic ordering**

Sort candidates by JSON name, index-path length, tagged before untagged, then index path. For each same-name group, select the first candidate unless the first two have equal depth and equal `Tagged`; in that case omit the entire name as ambiguous. Sort winners lexicographically by index path before returning.

- [ ] **Step 8: Remove old tuple parser and run focused tests**

Delete old `(string, bool, bool)` `parseJSONTag` from `tags.go`.

```bash
go test ./internal/schema -run 'Test(ParseJSONTagMetadata|SchemaFieldSelectionMatchesEncodingJSON)' -count=1
```

Expected: parser PASS; selection may remain RED until `structSchema` consumes resolver output.

- [ ] **Step 9: Commit resolver checkpoint**

Commit `jsonfields.go`, parser removal, and resolver tests.

---

### Task 3: Integrate selected fields into `structSchema`

**Files:**
- Modify: `internal/schema/reflector.go`
- Test: `internal/schema/jsonfields_test.go`

**Interfaces:**
- Consumes: `resolveJSONFields(reflect.Type) []jsonField`.
- Preserves: `Registry.Ref` and component APIs.

- [ ] **Step 1: Replace linear traversal**

In `structSchema`, remove direct `NumField` flattening and iterate only selected fields:

```go
for _, selected := range resolveJSONFields(t) {
    field := selected.Field
    rules, err := validationrule.Parse(field.Type, field.Tag.Get("validate"))
    if err != nil {
        return nil, fmt.Errorf("field %s.%s: %w", t, field.Name, err)
    }
    child, err := r.schemaFor(field.Type, next)
    if err != nil {
        return nil, fmt.Errorf("field %s.%s: %w", t, field.Name, err)
    }
    schemaMap := map[string]any(*child)
    if err := applyFieldTags(schemaMap, field, rules); err != nil {
        return nil, fmt.Errorf("field %s.%s: %w", t, field.Name, err)
    }
    properties[selected.Name] = schemaMap
    if hasRuleKind(rules, validationrule.Required) {
        required = append(required, selected.Name)
    }
}
```

No recursive embedded flattening remains in `structSchema`.

- [ ] **Step 2: Add hidden-field failure regression**

```go
type hiddenBadLeft struct {
    Value chan int `json:"value"`
}

type hiddenBadRight struct {
    Value string `json:"value"`
}

type hiddenBadFixture struct {
    hiddenBadLeft
    hiddenBadRight
}
```

Test `Registry.Ref(reflect.TypeOf(hiddenBadFixture{}))` succeeds and selected properties omit `value`. Reuse the `$ref` component-name helper from Task 1 rather than adding a duplicate helper.

- [ ] **Step 3: Run GREEN field-selection suite**

```bash
go test ./internal/schema -run 'Test(SchemaFieldSelectionMatchesEncodingJSON|HiddenAmbiguousFieldsDoNotFailSchema)' -count=1
go test ./internal/schema -count=1
```

Expected: PASS.

- [ ] **Step 4: Commit integration checkpoint**

Commit resolver consumption and hidden-field regression.

---

### Task 4: RED→GREEN `json:",string"` wire schema

**Files:**
- Modify: `internal/schema/reflector_test.go`
- Modify: `internal/schema/tags.go`
- Modify: `internal/schema/reflector.go`

**Interfaces:**

```go
func quotedLexical(value any) (string, error)
func applyQuotedWireSchema(schema map[string]any, field reflect.StructField) error
```

- [ ] **Step 1: Add quoted fixture and RED test**

```go
type quotedSchemaFixture struct {
    Count   int64   `json:"count,string" validate:"gte=1,lte=9,oneof=1 2" example:"2"`
    Size    uint64  `json:"size,string"`
    Ratio   float64 `json:"ratio,string"`
    Enabled bool    `json:"enabled,string"`
    Name    string  `json:"name,string" validate:"oneof=admin user" example:"admin"`
    Maybe   *int    `json:"maybe,string"`
    Code    int     `json:"code,string" format:"numeric-id"`
    Values  []int   `json:"values,string"`
}
```

Assert string wire schemas for scalar quoted fields; no numeric bounds on `count`; `count` enum `[]any{"1", "2"}` and example `"2"`; `name` enum `[]any{"\"admin\"", "\"user\""}` and example `"\"admin\""`; nullable `maybe` is string-or-null; explicit `numeric-id` format is on the string branch; composite `values` remains array.

- [ ] **Step 2: Characterize actual wire tokens**

Marshal a populated fixture and decode to `map[string]json.RawMessage`. Assert:

```go
if string(raw["count"]) != `"2"` { ... }
if string(raw["enabled"]) != `"true"` { ... }
if string(raw["name"]) != `"\"admin\""` { ... }
```

- [ ] **Step 3: Run RED quoted test**

```bash
go test ./internal/schema -run TestQuotedStringSchemaMatchesEncodingJSON -count=1
```

Expected: assertion FAIL because current schema exposes numeric/bool wire types.

- [ ] **Step 4: Implement lexical conversion**

```go
func quotedLexical(value any) (string, error) {
    data, err := json.Marshal(value)
    if err != nil {
        return "", err
    }
    return string(data), nil
}
```

Add `encoding/json` import to `tags.go`.

- [ ] **Step 5: Implement quoted-wire replacement**

`applyQuotedWireSchema` runs after `applyFieldTags` and mutates the map in place:

1. capture `description`, parsed `example`, and `enum`;
2. lexicalize enum items and example via `quotedLexical`;
3. build `stringBranch := map[string]any{"type":"string"}`;
4. apply only explicit `field.Tag.Get("format")` to `stringBranch`;
5. put lexicalized enum on `stringBranch`;
6. delete all existing keys from the original map;
7. if the Go field is pointer-shaped, set `oneOf` to stringBranch + `{ "type":"null" }`; otherwise copy stringBranch keys to the original map;
8. restore `description` and lexicalized `example` at the outer schema level;
9. do not restore numeric bounds or type-derived numeric formats.

- [ ] **Step 6: Hook transform into `structSchema`**

```go
if selected.Quoted {
    if err := applyQuotedWireSchema(schemaMap, field); err != nil {
        return nil, fmt.Errorf("field %s.%s: %w", t, field.Name, err)
    }
}
```

- [ ] **Step 7: Run GREEN quoted and full schema tests**

```bash
go test ./internal/schema -run TestQuotedStringSchemaMatchesEncodingJSON -count=1
go test ./internal/schema -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit quoted-wire checkpoint**

Commit quoted tests, `tags.go`, and the `structSchema` hook.

---

### Task 5: RED→GREEN JSON-compatible map keys

**Files:**
- Modify: `internal/schema/reflector.go`
- Modify: `internal/schema/reflector_test.go`

**Interfaces:**

```go
func jsonMapKeySupported(t reflect.Type) bool
```

- [ ] **Step 1: Add key fixtures**

```go
type textMapKey struct{ N int }
func (k textMapKey) MarshalText() ([]byte, error) { return []byte(strconv.Itoa(k.N)), nil }

type pointerOnlyTextMapKey struct{ N int }
func (*pointerOnlyTextMapKey) MarshalText() ([]byte, error) { return []byte("x"), nil }

type namedStringKey string
```

- [ ] **Step 2: Add RED matrix**

Expect success for `map[string]int`, `map[namedStringKey]int`, `map[int64]int`, `map[uint64]int`, `map[uintptr]int`, `map[textMapKey]int`. Expect failure for `map[struct{ X int }]int` and `map[pointerOnlyTextMapKey]int`. Successful schemas must be object + `additionalProperties`.

- [ ] **Step 3: Run RED map test**

```bash
go test ./internal/schema -run TestRegistryJSONCompatibleMapKeys -count=1
```

Expected: FAIL for integer and value-receiver `TextMarshaler` keys under current string-only guard.

- [ ] **Step 4: Implement key representability**

```go
func jsonMapKeySupported(t reflect.Type) bool {
    switch t.Kind() {
    case reflect.String,
        reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
        reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
        return true
    default:
        return t.Implements(textMarshalerType)
    }
}
```

Do not use `reflect.PointerTo(t).Implements(textMarshalerType)`.

- [ ] **Step 5: Replace map guard and run GREEN suite**

Use `!jsonMapKeySupported(t.Key())` as rejection condition; keep object + `additionalProperties` output.

```bash
go test ./internal/schema -run TestRegistryJSONCompatibleMapKeys -count=1
go test ./internal/schema -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit map-key checkpoint**

Commit reflector change and tests.

---

### Task 6: Integrated behavior verification before version metadata

- [ ] **Step 1: Stress schema package**

```bash
go test ./internal/schema -shuffle=on -count=10
```

Expected: PASS.

- [ ] **Step 2: Run full tests and race**

```bash
go test ./... -shuffle=on -count=3
go test -race ./...
```

Expected: PASS.

- [ ] **Step 3: Verify OpenAPI golden intentionally**

```bash
UPDATE_GOLDEN=1 go test ./examples/users-api -run TestUsersAPIEndToEnd/openapi_golden -count=1
git diff -- testdata/users.openapi.golden.json
```

Keep any golden diff only when every change is directly explained by corrected JSON field selection or quoted wire shape.

- [ ] **Step 4: Run public API compatibility**

```bash
bash scripts/check-public-api.sh
```

Expected: PASS against stable `v2.0.2`, with no incompatible change and no exported addition.

- [ ] **Step 5: Require behavior-only branch CI**

Require Go 1.22–1.26, API compatibility, race, coverage, golden, both fuzz targets, benchmark, and vulnerability scan all green on the exact behavior-only head.

---

### Task 7: Prepare `v2.0.3` metadata

**Files:**
- Modify: `version.go`
- Modify: `version_test.go`
- Modify: `README.md`
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Update version contract**

Change `Version` and `TestStableVersion` from `2.0.2` to `2.0.3`.

- [ ] **Step 2: Update README current-release references**

Change current stable/install references from `v2.0.2` to `v2.0.3`; preserve historical migration examples.

- [ ] **Step 3: Add changelog entry**

```markdown
## [2.0.3] - 2026-08-12

### Fixed

- OpenAPI schema field selection now follows default `encoding/json` embedded-field promotion, pointer embedding, field-depth dominance, tagged-field preference, and ambiguity omission rules.
- `json:",string"` primitive fields now generate the JSON string wire shape, including nullable pointer handling and quoted enum/example values without invalid numeric bounds.
- Schema generation now accepts JSON-compatible integer and value-receiver `encoding.TextMarshaler` map keys while continuing to reject unsupported key types.

### Compatibility

- No exported API identifiers or signatures changed.
- Runtime request binding and validation behavior are unchanged.
- Go 1.22 remains the minimum supported version.
- No third-party runtime dependencies were added.
```

- [ ] **Step 4: Run release SemVer gate**

```bash
bash scripts/check-public-api.sh v2.0.3
```

Expected: PASS; if a minor release is required, stop and locate accidental exported API.

- [ ] **Step 5: Run full final branch CI**

All jobs must be green on the exact release-ready head; any fix requires a fresh full run.

- [ ] **Step 6: Commit metadata separately**

Keep behavior and release metadata independently reviewable before squash merge.

---

### Task 8: PR, merge, post-merge verification, guarded publication

- [ ] **Step 1: Review final diff against `main`**

Confirm only approved spec/plan, schema implementation/tests, intentional golden diff if any, and `v2.0.3` metadata changed. Confirm no `go.mod` dependency addition, public API addition, or unrelated feature.

- [ ] **Step 2: Open one ready-for-review PR**

PR body summarizes field selection, quoted wire schemas, map-key conformance, RED→GREEN evidence, API compatibility, and intended `v2.0.3` patch.

- [ ] **Step 3: Require review and PR CI clean**

Verify head unchanged, `mergeable=true`, no unresolved/requested-change review, and every PR-triggered job green.

- [ ] **Step 4: Squash merge with expected head SHA**

Record merged `main` SHA and verify `main` is identical.

- [ ] **Step 5: Require post-merge CI on exact SHA**

Do not publish while pending; require push-triggered CI `success`, including API compatibility.

- [ ] **Step 6: Confirm `v2.0.3` tag/release absent**

Never reuse, move, or force-update an existing release tag.

- [ ] **Step 7: Create `publish/v2.0.3` from exact verified `main` SHA**

Publish branch must equal `main` byte-for-byte and contain no release-only commit.

- [ ] **Step 8: Require guarded Release workflow**

The workflow must pass identity/version checks, full release quality gates, and `bash scripts/check-public-api.sh v2.0.3` before publication.

- [ ] **Step 9: Independently verify publication**

Verify tag `v2.0.3` targets exact verified `main` SHA; latest release is `oashttp v2.0.3`, `draft=false`, `prerelease=false`; `version.go` at tag says `2.0.3`; tag and `main` are identical immediately after publication.

- [ ] **Step 10: Record completion evidence**

Final report cites PR merge SHA, post-merge CI, release workflow, tag target, and release metadata. Branch CI alone is not completion evidence.
