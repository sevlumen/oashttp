# Schema ↔ encoding/json Conformance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make generated schemas match default `encoding/json` v1 wire semantics for embedded-field selection, field dominance/conflicts, `json:",string"`, and JSON-compatible map keys, then publish the patch as `v2.0.3` if all compatibility and release gates pass.

**Architecture:** Add a focused `internal/schema/jsonfields.go` resolver that converts a Go struct into deterministic selected JSON fields before schema generation. Keep field selection independent from schema reflection, then apply a narrow quoted-wire transform for `json:",string"`; map-key representability remains a small reflector helper. Runtime binding/validation are unchanged.

**Tech Stack:** Go 1.22–1.26, standard-library `encoding/json`, `encoding.TextMarshaler`, `reflect`, existing `internal/schema`, existing `internal/validationrule`, GitHub Actions, existing public-API compatibility/release gates.

## Global Constraints

- Target default/legacy `encoding/json` v1 behavior supported by Go 1.22–1.26; do not implement experimental `encoding/json/v2` semantics.
- No exported `oashttp` API additions or signature changes.
- No third-party runtime dependencies; `go.mod` must remain dependency-free at runtime.
- Do not change runtime binding or validation traversal.
- `validate:"required"` remains the source of OpenAPI `required`; `omitempty` and `omitzero` do not redefine it.
- General `json.Marshaler` inference, interface schema policy, and byte-slice special encoding are out of scope.
- `SchemaProvider` remains the explicit escape hatch for custom JSON representations.
- Hidden or ambiguous fields must not be reflected, validated for schema purposes, or allowed to fail schema generation.
- Characterization and implementation follow RED → GREEN; temporary experimental mutations must not remain in the final PR diff.
- If the final diff remains patch-compatible, release target is `v2.0.3` from a verified `main` commit through `publish/v2.0.3`.

---

## File map

- Create `internal/schema/jsonfields.go`: JSON tag metadata, field discovery, embedded traversal, dominance/conflict resolution, quoted applicability.
- Create `internal/schema/jsonfields_test.go`: resolver and `encoding/json` field-selection conformance matrix.
- Modify `internal/schema/reflector.go`: consume resolved fields; accept JSON-compatible map keys.
- Modify `internal/schema/tags.go`: quoted-wire schema transformation and lexical conversion for enum/example values.
- Modify `internal/schema/reflector_test.go`: schema-level `,string` and map-key contract tests.
- Modify `version.go`, `version_test.go`, `README.md`, `CHANGELOG.md`: release-ready `v2.0.3` metadata only after behavior is green.
- Reuse `.github/workflows/ci.yml`, `.github/workflows/release.yml`, and `scripts/check-public-api.sh` without changing them unless final verification finds a release-gate defect unrelated to schema behavior.

---

### Task 1: Create the implementation branch and RED field-selection conformance matrix

**Files:**
- Create: `internal/schema/jsonfields_test.go`
- No production file changes in this task.

**Interfaces:**
- Consumes: existing `Registry.Ref(reflect.Type)` and generated component schemas.
- Produces: failing tests that define selected JSON property names according to `encoding/json`.

- [ ] **Step 1: Create the implementation branch from the approved design/plan head**

Create `fix/schema-json-conformance` from the final `design/schema-json-conformance` head so the approved spec and plan travel with the implementation PR.

- [ ] **Step 2: Add reusable test helpers**

Create `internal/schema/jsonfields_test.go` with these helpers:

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
    name := componentNameFromRef(t, *ref)
    component := registry.Components()[name]
    if component == nil {
        t.Fatalf("missing component %q", name)
    }
    properties := (*component)["properties"].(map[string]any)
    keys := make([]string, 0, len(properties))
    for key := range properties {
        keys = append(keys, key)
    }
    sort.Strings(keys)
    return keys
}

func componentNameFromRef(t *testing.T, schema map[string]any) string {
    t.Helper()
    ref, ok := schema["$ref"].(string)
    if !ok {
        t.Fatalf("schema has no $ref: %#v", schema)
    }
    const prefix = "#/components/schemas/"
    if len(ref) <= len(prefix) || ref[:len(prefix)] != prefix {
        t.Fatalf("unexpected $ref %q", ref)
    }
    return ref[len(prefix):]
}
```

- [ ] **Step 3: Add fixtures for embedded selection and dominance**

Use concrete fixtures in the same test file:

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

type jsonOuterShadow struct {
    Value string `json:"value"`
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
    Named jsonNamedEmbedded `json:"named"`
    Skip string `json:"-"`
    Invalid string `json:"bad\\name"`
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

The anonymous fixture types are intentionally package-private: default `encoding/json` may traverse an unexported anonymous struct to expose exported members.

- [ ] **Step 4: Add the RED oracle test**

```go
func TestSchemaFieldSelectionMatchesEncodingJSON(t *testing.T) {
    pointer := &jsonEmbeddedPointer{PointerValue: "p"}
    cases := []any{
        jsonSelectionFixture{
            jsonEmbedded:        jsonEmbedded{Promoted: "a"},
            jsonEmbeddedPointer: pointer,
            Named:               jsonNamedEmbedded{Value: "n"},
            Skip:                "skip",
            Invalid:             "fallback",
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

- [ ] **Step 5: Run the RED test**

Run:

```bash
go test ./internal/schema -run TestSchemaFieldSelectionMatchesEncodingJSON -count=1
```

Expected: FAIL on at least anonymous `*struct` promotion and dominance/ambiguity cases under the current linear `structSchema` implementation. The failure must be assertion-based, not compile/setup failure.

- [ ] **Step 6: Commit the test-only RED checkpoint**

Commit only `internal/schema/jsonfields_test.go` plus the already-approved spec/plan history. Record the CI run that demonstrates the expected RED behavior before production code is added.

---

### Task 2: Implement JSON tag parsing and deterministic field resolution

**Files:**
- Create: `internal/schema/jsonfields.go`
- Modify: `internal/schema/tags.go`
- Test: `internal/schema/jsonfields_test.go`

**Interfaces:**
- Produces:

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

- [ ] **Step 1: Add focused parser tests before implementation**

Add table-driven assertions to `jsonfields_test.go` for:

```go
func TestParseJSONTagMetadata(t *testing.T) {
    typ := reflect.TypeOf(struct {
        Plain   string
        Named   int    `json:"named,omitempty,string"`
        Options int    `json:",omitempty,string"`
        Skip    string `json:"-"`
        Invalid string `json:"bad\\name"`
    }{})

    cases := []struct {
        field int
        want  jsonTag
    }{
        {0, jsonTag{}},
        {1, jsonTag{Name: "named", HasName: true, OmitEmpty: true, String: true}},
        {2, jsonTag{OmitEmpty: true, String: true}},
        {3, jsonTag{Skip: true}},
        {4, jsonTag{}},
    }
    for _, tc := range cases {
        if got := parseJSONTag(typ.Field(tc.field)); !reflect.DeepEqual(got, tc.want) {
            t.Fatalf("field %d tag=%+v want=%+v", tc.field, got, tc.want)
        }
    }
}
```

- [ ] **Step 2: Run parser test to verify RED**

Run:

```bash
go test ./internal/schema -run TestParseJSONTagMetadata -count=1
```

Expected: FAIL to compile until the new `jsonTag` API replaces the old tuple parser.

- [ ] **Step 3: Implement `jsonTag`, valid-name checking, and quoted applicability**

Create `internal/schema/jsonfields.go`. `validJSONTagName` must accept non-empty names composed only of Unicode letters/digits or legacy JSON-tag punctuation, and reject quote, backslash, comma, control characters, and whitespace. `parseJSONTag` must:

1. return `Skip=true` only for the exact name `-`;
2. validate the explicit name before setting `Name`/`HasName`;
3. parse `omitempty` and `string` independently;
4. leave an invalid explicit name equivalent to no explicit name.

`quotedJSONType` dereferences at most one or more pointer layers until the base type and returns true only for bool, string, signed integers, unsigned integers including `uintptr`, and floats.

- [ ] **Step 4: Implement breadth-first candidate discovery**

Use an internal queue entry:

```go
type jsonFieldLevel struct {
    Type      reflect.Type
    Index     []int
    Depth     int
    Ancestors map[reflect.Type]bool
}
```

Start with the root struct at depth 0. For each exported field or anonymous struct candidate:

- parse the tag before visibility decisions;
- skip `json:"-"`;
- dereference an anonymous pointer once for embedding decisions;
- ignore unexported ordinary fields;
- ignore unexported anonymous non-struct fields;
- enqueue an anonymous struct/`*struct` without a valid explicit JSON name at `Depth+1` when that struct is not already in the current ancestry path;
- otherwise append a `jsonField` candidate with a full copied index path, selected name, `Tagged=tag.HasName`, and `Quoted=tag.String && quotedJSONType(field.Type)`.

Do not globally mark embedded struct types as visited: two sibling embeddings of the same type must both be allowed to reach the dominance phase.

- [ ] **Step 5: Implement dominance and deterministic ordering**

Group candidates by `Name`. For each group:

1. find minimum index depth (`len(Index)` is sufficient because each embedded hop adds one index);
2. keep only candidates at that depth;
3. if any remaining candidate has `Tagged=true`, discard untagged candidates;
4. select exactly one remaining candidate; omit the name if multiple remain;
5. sort selected fields lexicographically by their integer `Index` paths so declaration-oriented output is deterministic.

- [ ] **Step 6: Remove the old tuple `parseJSONTag` from `tags.go`**

Delete the existing `(string, bool, bool)` parser. Keep `applyFieldTags`, validation-bound helpers, and example parsing in `tags.go` for now.

- [ ] **Step 7: Run parser/resolver tests**

Run:

```bash
go test ./internal/schema -run 'Test(ParseJSONTagMetadata|SchemaFieldSelectionMatchesEncodingJSON)' -count=1
```

Expected: parser test PASS; selection test may still FAIL because `structSchema` has not yet been switched to the resolver.

- [ ] **Step 8: Commit the resolver implementation**

Commit `internal/schema/jsonfields.go`, `internal/schema/tags.go`, and the parser/resolver test additions.

---

### Task 3: Make `structSchema` consume resolved JSON fields

**Files:**
- Modify: `internal/schema/reflector.go`
- Test: `internal/schema/jsonfields_test.go`
- Test: `internal/schema/reflector_test.go`

**Interfaces:**
- Consumes: `resolveJSONFields(reflect.Type) []jsonField`.
- Preserves: existing `Registry.Ref` and component APIs.

- [ ] **Step 1: Replace linear field traversal with resolved fields**

In `structSchema`, remove the direct `for i := 0; i < t.NumField(); i++` flattening logic. Iterate:

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

Do not recursively reflect unselected embedded fields in `structSchema`; resolver selection is now the only field-selection source of truth.

- [ ] **Step 2: Add a hidden-invalid-field regression**

Add a fixture where two equal-depth embedded fields conflict on the same JSON name and one hidden field has an unsupported type or invalid validation rule:

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

Test that `Registry.Ref(reflect.TypeOf(hiddenBadFixture{}))` succeeds and the ambiguous `value` property is absent. This proves hidden fields cannot fail schema generation.

- [ ] **Step 3: Run the field-selection and hidden-field tests**

Run:

```bash
go test ./internal/schema -run 'Test(SchemaFieldSelectionMatchesEncodingJSON|HiddenAmbiguousFieldsDoNotFailSchema)' -count=1
```

Expected: PASS.

- [ ] **Step 4: Run all schema tests**

Run:

```bash
go test ./internal/schema -count=1
```

Expected: PASS before moving to `,string` work.

- [ ] **Step 5: Commit field-selection integration**

Commit only resolver integration and its regression tests.

---

### Task 4: RED→GREEN `json:",string"` wire-schema conformance

**Files:**
- Modify: `internal/schema/reflector_test.go`
- Modify: `internal/schema/tags.go`
- Consume: `jsonField.Quoted` from `internal/schema/jsonfields.go`
- Modify: `internal/schema/reflector.go`

**Interfaces:**
- Produces:

```go
func applyQuotedWireSchema(schema map[string]any, field reflect.StructField) error
func quotedLexical(value any) (string, error)
```

- [ ] **Step 1: Add a quoted-wire RED fixture**

Add:

```go
type quotedSchemaFixture struct {
    Count   int64    `json:"count,string" validate:"gte=1,lte=9,oneof=1 2" example:"2"`
    Size    uint64   `json:"size,string"`
    Ratio   float64  `json:"ratio,string"`
    Enabled bool     `json:"enabled,string"`
    Name    string   `json:"name,string" validate:"oneof=admin user" example:"admin"`
    Maybe   *int     `json:"maybe,string"`
    Values  []int    `json:"values,string"`
}
```

Add assertions that generated schemas for `count`, `size`, `ratio`, `enabled`, and `name` describe string wire values; `count` has no `minimum`/`maximum`, its enum is `[]any{"1", "2"}`, and example is `"2"`; `name` enum is `[]any{"\"admin\"", "\"user\""}` because legacy `,string` encodes a Go string as JSON text inside a JSON string; `maybe` has `oneOf` containing string and null; `values` remains an array because `,string` does not apply to composite types.

- [ ] **Step 2: Characterize actual JSON output in the same test**

Marshal a populated `quotedSchemaFixture` with `encoding/json` and decode into `map[string]json.RawMessage`. Assert representative raw values:

```go
if string(raw["count"]) != `"2"` { ... }
if string(raw["enabled"]) != `"true"` { ... }
if string(raw["name"]) != `"\"admin\""` { ... }
```

This keeps the expected schema lexical form tied to the standard-library wire behavior.

- [ ] **Step 3: Run the RED quoted test**

Run:

```bash
go test ./internal/schema -run TestQuotedStringSchemaMatchesEncodingJSON -count=1
```

Expected: FAIL because the current schema still exposes numeric/bool types and keywords.

- [ ] **Step 4: Implement deterministic lexical conversion**

In `tags.go`:

```go
func quotedLexical(value any) (string, error) {
    data, err := json.Marshal(value)
    if err != nil {
        return "", err
    }
    return string(data), nil
}
```

Use `encoding/json` itself to define the inner lexical JSON text: integers become `1`, booleans `true`, floats their JSON form, strings `"admin"` including quotes in the resulting Go string.

- [ ] **Step 5: Implement `applyQuotedWireSchema`**

Call it only when `selected.Quoted` is true, after `applyFieldTags` has produced annotations/validation keywords.

Behavior:

1. read and preserve `description` and the raw example/enum values already present;
2. lexicalize each enum item with `quotedLexical`;
3. lexicalize the parsed example with `quotedLexical`;
4. build a fresh non-null branch `map[string]any{"type": "string"}`;
5. if `field.Tag.Get("format")` is non-empty, apply that explicit format to the string branch; do not preserve type-derived numeric formats;
6. put lexicalized enum on the string branch;
7. if the Go field is pointer-shaped, replace the schema with `oneOf: []any{stringBranch, map[string]any{"type":"null"}}`; otherwise replace it with the string branch keys;
8. restore `description` and lexicalized `example` at the outer field-schema level;
9. do not copy `minimum`, `maximum`, numeric formats, or other keywords invalid for the quoted wire type.

The transform must mutate the provided `map[string]any` in place by deleting existing keys before writing the replacement, because callers hold that map reference.

- [ ] **Step 6: Wire quoted transformation into `structSchema`**

After `applyFieldTags(schemaMap, field, rules)`, add:

```go
if selected.Quoted {
    if err := applyQuotedWireSchema(schemaMap, field); err != nil {
        return nil, fmt.Errorf("field %s.%s: %w", t, field.Name, err)
    }
}
```

- [ ] **Step 7: Run quoted tests and full schema package**

Run:

```bash
go test ./internal/schema -run TestQuotedStringSchemaMatchesEncodingJSON -count=1
go test ./internal/schema -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit quoted wire semantics**

Commit `tags.go`, `reflector.go`, and quoted tests as one independently reviewable behavior correction.

---

### Task 5: RED→GREEN JSON-compatible map-key support

**Files:**
- Modify: `internal/schema/reflector.go`
- Modify: `internal/schema/reflector_test.go`

**Interfaces:**
- Produces:

```go
func jsonMapKeySupported(t reflect.Type) bool
```

- [ ] **Step 1: Add map-key fixtures and RED tests**

Add:

```go
type textMapKey struct{ N int }
func (k textMapKey) MarshalText() ([]byte, error) { return []byte(strconv.Itoa(k.N)), nil }

type pointerOnlyTextMapKey struct{ N int }
func (*pointerOnlyTextMapKey) MarshalText() ([]byte, error) { return []byte("x"), nil }

type namedStringKey string
```

Test successful `Registry.Ref` for:

```go
map[string]int{}
map[namedStringKey]int{}
map[int64]int{}
map[uint64]int{}
map[uintptr]int{}
map[textMapKey]int{}
```

and failure for:

```go
map[struct{ X int }]int{}
map[pointerOnlyTextMapKey]int{}
```

For every success case, assert `type == "object"` and `additionalProperties` exists.

- [ ] **Step 2: Run map-key test to verify RED**

Run:

```bash
go test ./internal/schema -run TestRegistryJSONCompatibleMapKeys -count=1
```

Expected: FAIL for integer and `TextMarshaler` key types under the current string-only check.

- [ ] **Step 3: Implement `jsonMapKeySupported`**

In `reflector.go`:

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

Do not use `reflect.PointerTo(t).Implements(textMarshalerType)` for map keys; a pointer-only receiver on a non-pointer map key does not satisfy the map-key contract.

- [ ] **Step 4: Replace the string-only map guard**

Change the map branch in `schemaFor` to reject only when `!jsonMapKeySupported(t.Key())`; keep the generated wire schema as object + `additionalProperties` because all JSON object keys are strings on the wire.

- [ ] **Step 5: Run map-key and full schema tests**

Run:

```bash
go test ./internal/schema -run TestRegistryJSONCompatibleMapKeys -count=1
go test ./internal/schema -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit map-key conformance**

Commit only the map-key helper/change and its tests.

---

### Task 6: Full behavior verification before release metadata

**Files:**
- No new behavior files unless verification reveals a defect.

**Interfaces:**
- Verifies all prior tasks as one integrated tree before changing version metadata.

- [ ] **Step 1: Run focused schema package repeatedly**

```bash
go test ./internal/schema -shuffle=on -count=10
```

Expected: PASS.

- [ ] **Step 2: Run full repository tests and race detector**

```bash
go test ./... -shuffle=on -count=3
go test -race ./...
```

Expected: PASS.

- [ ] **Step 3: Verify generated OpenAPI output intentionally**

```bash
UPDATE_GOLDEN=1 go test ./examples/users-api -run TestUsersAPIEndToEnd/openapi_golden -count=1
git diff -- testdata/users.openapi.golden.json
```

If there is no affected fixture, expect no golden diff. If there is a diff, inspect every changed property and keep it only when directly explained by corrected `encoding/json` field selection/wire semantics; otherwise treat it as a regression and fix the implementation.

- [ ] **Step 4: Run public API compatibility helper**

```bash
bash scripts/check-public-api.sh
```

Expected: PASS against stable `v2.0.2`, with no incompatible change and no exported addition.

- [ ] **Step 5: Require branch CI to pass on the behavior-only head**

Require Go 1.22–1.26, public API compatibility, race, coverage, OpenAPI golden, both fuzz targets, benchmark, and vulnerability scan to complete successfully before version metadata changes.

---

### Task 7: Prepare patch release metadata for `v2.0.3`

**Files:**
- Modify: `version.go`
- Modify: `version_test.go`
- Modify: `README.md`
- Modify: `CHANGELOG.md`

**Interfaces:**
- Consumes: behavior-only green branch.
- Produces: release-ready source whose public `Version`, README installation/version text, and changelog agree on `2.0.3`.

- [ ] **Step 1: Update `Version` and its contract test**

Change:

```go
const Version = "2.0.2"
```

to:

```go
const Version = "2.0.3"
```

and update `TestStableVersion` to expect `"2.0.3"`.

- [ ] **Step 2: Update README stable release/install references**

Replace stable `v2.0.2` references that identify the current installable release with `v2.0.3`. Do not rewrite historical migration/version examples that intentionally refer to older releases.

- [ ] **Step 3: Add changelog section**

Insert above `2.0.2`:

```markdown
## [2.0.3] - 2026-08-12

### Fixed

- OpenAPI schema field selection now follows default `encoding/json` embedded-field promotion, pointer embedding, field-depth dominance, tagged-field preference, and ambiguity omission rules.
- `json:",string"` primitive fields now generate the JSON string wire shape, including nullable pointer handling and type-correct quoted enum/example values without invalid numeric bounds.
- Schema generation now accepts JSON-compatible integer and value-receiver `encoding.TextMarshaler` map keys while continuing to reject unsupported key types.

### Compatibility

- No exported API identifiers or signatures changed.
- Runtime request binding and validation behavior are unchanged.
- Go 1.22 remains the minimum supported version.
- No third-party runtime dependencies were added.
```

Keep `[Unreleased]` empty unless another unrelated change has landed on `main` before integration; if so, rebase/merge-base review must separate that work rather than silently absorbing it.

- [ ] **Step 4: Run release-mode SemVer check**

```bash
bash scripts/check-public-api.sh v2.0.3
```

Expected: PASS. If it suggests/requires a minor release, stop release preparation and inspect for an accidental exported addition before proceeding.

- [ ] **Step 5: Run final branch CI on the exact release-ready head**

Require all CI jobs to pass. Do not add further code changes after this head except review fixes followed by a fresh full CI run.

- [ ] **Step 6: Commit release metadata separately**

Use a dedicated release-prep commit so behavior fixes and version metadata remain independently reviewable before squash merge.

---

### Task 8: PR review, merge, post-merge verification, and guarded `v2.0.3` publication

**Files:**
- No source changes unless review/CI finds a defect.

**Interfaces:**
- Consumes: final branch head from Task 7.
- Produces: reviewed `main`, then tag/GitHub Release `v2.0.3` pointing at the exact verified `main` SHA.

- [ ] **Step 1: Review the final diff against `main`**

Verify the changed-file set is limited to the approved spec/plan, schema implementation/tests, and release metadata. Confirm no `go.mod` dependency addition, no public API addition, and no unrelated runtime feature.

- [ ] **Step 2: Open one PR to `main`**

Create a ready-for-review PR only after the final branch CI is green. PR body must summarize:

- corrected `encoding/json` field selection;
- quoted primitive wire schemas;
- map-key conformance;
- RED→GREEN evidence;
- public-API compatibility result;
- intended patch release `v2.0.3`.

- [ ] **Step 3: Require PR integration CI and review to be clean**

Before merge, verify:

- PR head SHA has not moved since the green branch run unless the new head has its own fresh green run;
- `mergeable=true`;
- no unresolved review thread or requested-change review;
- PR-triggered CI passes Go 1.22–1.26, API compatibility, quality suite, fuzzing, benchmarks, and vulnerability scan.

- [ ] **Step 4: Squash merge with expected head SHA**

Merge only after Step 3. Record the resulting `main` SHA and verify `main` is identical to it.

- [ ] **Step 5: Require post-merge `main` CI on the exact merge SHA**

Do not publish while this run is pending. Require the push-triggered CI run for the exact merge SHA to complete `success`, including the public API compatibility job.

- [ ] **Step 6: Verify the release tag does not already exist**

Confirm `v2.0.3` is absent from tags/releases. Never reuse, move, or force-update a release tag.

- [ ] **Step 7: Create `publish/v2.0.3` from the exact verified `main` SHA**

The publish branch must contain no release-only commit and must equal `main` byte-for-byte.

- [ ] **Step 8: Require the guarded Release workflow to pass**

The workflow must verify branch identity, `Version`, all release quality gates, and:

```bash
bash scripts/check-public-api.sh v2.0.3
```

before the publication step executes.

- [ ] **Step 9: Independently verify publication**

After workflow success, verify all of:

- tag `v2.0.3` points at the exact verified `main` SHA;
- latest GitHub Release is `oashttp v2.0.3`, `draft=false`, `prerelease=false`;
- `version.go` at tag says `2.0.3`;
- tag and `main` are identical immediately after publication.

- [ ] **Step 10: Record completion evidence**

Final report must cite the PR merge SHA, post-merge CI, release workflow, tag target, and release metadata. Do not claim completion from branch CI alone.
