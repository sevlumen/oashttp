# Schema ↔ encoding/json Conformance Design

## Status

Approved design, pending written-spec review before implementation planning.

## Goal

Make `internal/schema` describe the JSON wire shape selected by Go's default `encoding/json` behavior for the identified drift areas: embedded-field selection, JSON field-name dominance/conflicts, `json:",string"`, and JSON-compatible map key types.

This is a correctness hardening of generated OpenAPI/JSON Schema behavior. It does not add exported oashttp API.

## Compatibility target

The target is the default/legacy `encoding/json` behavior used by the repository's supported Go 1.22–1.26 toolchains, not the experimental/new `encoding/json/v2` semantics.

The field resolver must follow these v1-compatible rules:

- exported ordinary fields are candidates unless `json:"-"`;
- anonymous embedded structs promote their eligible fields when they have no explicit JSON name;
- anonymous `*struct` fields are treated the same way for field selection;
- an anonymous field with an explicit JSON name is a normal named field and is not promoted;
- unexported non-embedded fields are ignored;
- an unexported anonymous struct or `*struct` may still be traversed because it can expose exported members;
- invalid explicit JSON tag names fall back to the Go field name rather than becoming arbitrary schema property names;
- at a given JSON name, the shallowest depth wins;
- if multiple candidates exist at that shallowest depth and at least one is explicitly JSON-named, only explicitly named candidates remain;
- exactly one remaining candidate wins; otherwise the name is ambiguous and omitted without error.

The resolver is an independent implementation of documented behavior. It must not copy or depend on unexported `encoding/json` internals.

## Current drift

`structSchema` currently walks fields linearly, skips all unexported fields before considering anonymous-struct promotion, promotes only direct anonymous structs, and writes flattened properties directly into a map. This means anonymous `*struct`, field dominance, equal-depth ambiguity, tagged-vs-untagged conflicts, and unexported anonymous struct promotion can diverge from `encoding/json`.

`parseJSONTag` currently recognizes `omitempty` but does not expose `string` metadata or validate explicit JSON names.

Map schemas currently accept only string keys even though `encoding/json` also accepts signed integer keys, unsigned integer keys including `uintptr`, and key types that themselves implement `encoding.TextMarshaler`.

## Scope

### In scope

1. Add an internal JSON-field resolver under `internal/schema`.
2. Make `structSchema` consume resolved JSON fields instead of flattening anonymous structs itself.
3. Match embedded struct and embedded pointer-to-struct field selection rules.
4. Match shallowest-depth/tagged-field/ambiguity dominance rules.
5. Validate JSON tag names with the same accepted-name character class as default `encoding/json`; invalid names fall back to the Go field name.
6. Recognize `json:",string"` for bool, string, integer, unsigned integer, `uintptr`, float, and pointers to those primitive kinds.
7. Make schema wire type `string` when the v1 `string` option applies.
8. Allow map keys whose kinds are string, signed integer, unsigned integer/`uintptr`, or whose key type itself implements `encoding.TextMarshaler`.
9. Add conformance tests that characterize behavior through `encoding/json`, not only through an implementation-shaped unit test.
10. Preserve zero runtime dependencies and all current public API.

### Explicitly out of scope

- General inference from `json.Marshaler` implementations. `SchemaProvider` remains the explicit escape hatch for custom representations.
- Experimental `encoding/json/v2` options or semantics.
- Changing runtime binding or validation traversal.
- Redefining `validate:"required"`, `omitempty`, or `omitzero` semantics.
- Interface-field schema policy; unsupported interface types remain unsupported unless already handled by another explicit provider.
- Other JSON representation differences not part of the approved field-selection/string/map-key batch, including byte-slice special encoding. Those should be handled by a separate focused conformance item rather than expanding this resolver change.

## Architecture

Add `internal/schema/jsonfields.go` as a narrow dependency-free resolver.

```text
reflect.Type (struct)
        |
        v
resolveJSONFields
        |
        +-- visibility / json:"-"
        +-- tag parsing + valid name
        +-- embedded struct / *struct traversal
        +-- depth + explicit-name metadata
        +-- dominance / ambiguity resolution
        +-- json:",string" applicability
        |
        v
[]jsonField
        |
        v
Registry.structSchema
        |
        +-- schemaFor(field.Type)
        +-- field annotations / validation-derived schema
        +-- wire-shape adjustment for ,string
        +-- properties / required
```

### `jsonField`

Use an unexported representation similar to:

```go
type jsonField struct {
    Field   reflect.StructField
    Index   []int
    Name    string
    Tagged  bool
    Quoted  bool
}
```

`Index` records the field path through embedded structs. `Tagged` means an explicit valid JSON name was supplied; options-only tags such as `json:",omitempty"` are not tagged for dominance purposes. `Quoted` means legacy `json:",string"` applies to the field's top-level primitive representation.

No runtime code outside `internal/schema` consumes this type.

## Field discovery and dominance

Discovery is breadth-first by embedding depth. The resolver records candidate fields rather than writing properties immediately.

For each field:

1. Skip `json:"-"`.
2. Parse name/options.
3. If the explicit name is invalid, treat it as empty and use normal field-name/promotion behavior.
4. Ignore an unexported ordinary field.
5. For an anonymous field, dereference one pointer for field-selection purposes.
6. Ignore an unexported anonymous non-struct field.
7. If an anonymous struct/`*struct` has no explicit valid JSON name, enqueue its struct fields at the next depth rather than emitting the embedded field itself.
8. Otherwise record the field as a candidate with its selected JSON name.

After discovery, candidates are grouped by JSON name. For each name:

1. Keep only candidates at the minimum depth.
2. If any minimum-depth candidate has an explicit valid JSON name, discard untagged candidates at that depth.
3. If exactly one candidate remains, select it.
4. If multiple remain, omit the JSON name entirely.

Selected fields are returned in deterministic field-index order so schema `required` output remains stable and declaration-oriented rather than depending on map iteration.

Recursive anonymous embedding must terminate safely. The resolver may track already-expanded struct types per BFS path/depth, but it must not suppress legitimate repeated embeddings that participate in a dominance conflict.

## JSON tag parsing

Replace the current tuple-only parsing with metadata sufficient for selection and wire shape, for example:

```go
type jsonTag struct {
    Name      string
    HasName   bool
    OmitEmpty bool
    String    bool
    Skip      bool
}
```

Only data actually needed by schema generation must affect behavior. `OmitEmpty` may remain parsed for compatibility with existing helpers, but omission options do not determine OpenAPI `required`; `validate:"required"` remains the source of that framework-level constraint.

A JSON tag name is valid when non-empty and composed of Unicode letters/digits or the punctuation accepted by legacy `encoding/json`, excluding quote, backslash, and comma. An invalid explicit name behaves as no explicit name.

## `json:",string"` wire schema

Legacy `encoding/json` stringification applies when the field's top-level type is one of:

- bool;
- signed integer;
- unsigned integer including `uintptr`;
- float;
- string;
- pointer to one of the above.

It does not recursively stringify composite types.

For a quoted field, the generated field schema must describe the JSON wire value as a string. The implementation should perform the wire-shape adjustment after the underlying Go schema is known, while preventing incompatible numeric/boolean JSON-Schema keywords from being left on a string schema.

Rules for this batch:

- resulting wire `type` is `string`;
- pointer nullability remains representable: a quoted pointer field accepts JSON `null` as well as the quoted string representation;
- type-specific numeric formats such as `int32`, `int64`, `float`, or `double` are removed from the wire string schema unless an explicit field `format` annotation subsequently sets a string format;
- numeric bounds (`minimum`, `maximum`) are omitted because they do not constrain JSON strings correctly;
- validation-derived `enum` values are converted to their wire-string lexical representation when this can be done unambiguously for the supported primitive type; otherwise the enum is omitted rather than emitting an incorrect contract;
- `description` remains valid and is preserved;
- examples must describe the wire string representation when an `example` tag is present.

This changes documentation shape only; decoding and validation execution remain unchanged.

## Map key compatibility

A map is JSON-object representable when its key type satisfies one of:

```text
kind == string
kind in signed integers
kind in unsigned integers, including uintptr
keyType implements encoding.TextMarshaler
```

The last rule is about the key type itself. A non-pointer key type that only gains `MarshalText` through a pointer receiver is not sufficient, matching `encoding/json` map-key behavior.

The generated schema remains:

```json
{
  "type": "object",
  "additionalProperties": { ...value schema... }
}
```

because JSON object keys are strings on the wire regardless of the Go map key type.

Unsupported map key types continue to fail schema compilation with a clear error.

## Validation interaction

This change does not alter `internal/validation` traversal or rule execution.

For selected fields, `validate:"required"` continues to control OpenAPI `required` membership. A field eliminated by JSON dominance/ambiguity cannot appear in `properties` or `required`, even if its Go field carries validation tags, because it has no selected JSON member under `encoding/json`.

For quoted primitive fields, validation remains runtime-side after JSON decoding. Schema keywords that would be type-invalid on the string wire representation must not be emitted merely to preserve previous numeric-schema behavior.

## Error handling

The resolver itself should not return an error for ordinary JSON field conflicts; ambiguity means omission, matching `encoding/json`.

Schema generation still returns errors for unsupported selected field types, invalid validation rules, unsupported map key types, nil schema providers, or existing reflector failures.

If a conflicting/hidden Go field is not selected by JSON rules, its unsupported type or invalid schema annotations must not fail schema generation because `encoding/json` would not expose that field on the wire.

## Testing strategy

Implementation follows RED → GREEN.

### Resolver/struct conformance matrix

Add table-driven fixtures covering at least:

- direct anonymous embedded struct promotion;
- anonymous `*struct` promotion;
- unexported anonymous struct exposing an exported member;
- anonymous struct with explicit JSON name not promoted;
- outer field shadowing a deeper embedded field;
- two equal-depth untagged fields with the same JSON name => omitted;
- one tagged and one untagged equal-depth field => tagged wins;
- two tagged equal-depth fields with the same JSON name => omitted;
- `json:"-"` exclusion;
- invalid JSON tag name falling back to the Go field name;
- options-only JSON tag not counting as explicitly named for dominance.

For populated fixtures, use `encoding/json.Marshal` as an oracle for observable object-member selection and compare its keys with generated schema property keys where possible.

### `,string` matrix

Characterize JSON output and schema for:

- int64;
- uint64/`uintptr` representative;
- float64;
- bool;
- string;
- pointer to primitive;
- composite field with `,string` showing that the option does not recursively change an array/slice/map/struct schema.

Include at least one validation-tag case to prove incompatible numeric keywords are not left on a wire-string schema.

### Map-key matrix

Expect success for:

- `map[string]T`;
- named string key;
- signed integer key;
- unsigned integer/`uintptr` key;
- key type with value-receiver `MarshalText`.

Expect failure for:

- struct key with no `MarshalText`;
- key type where only `*K` implements `MarshalText` but `K` is the map key.

### Existing suite

All existing schema tests, OpenAPI golden tests, external public-facade contract tests, Go 1.22–1.26 matrix, race, coverage, fuzz, benchmarks, vulnerability scan, and public API compatibility gate must remain green.

If generated OpenAPI golden output changes, update it only when the diff is an intentional consequence of corrected JSON field selection/wire shape.

## Files expected to change

Primary:

- `internal/schema/jsonfields.go` (new)
- `internal/schema/reflector.go`
- `internal/schema/tags.go`
- `internal/schema/reflector_test.go`

Possible focused test split if the matrix becomes large:

- `internal/schema/jsonfields_test.go` (new)

Release/documentation files are changed only after implementation is green and the final diff proves this is a patch-compatible behavior correction.

## SemVer and release completion

The latest stable release at design time is `v2.0.2`. If the final implementation remains free of exported API additions and the compatibility gate accepts the proposed patch, the intended release is `v2.0.3`.

Before publication:

1. merge the implementation PR to `main`;
2. require post-merge `main` CI to pass;
3. update `Version`, README, and changelog consistently through the normal release-ready PR flow if not already included in the implementation release preparation;
4. run the release compatibility gate against `v2.0.3`;
5. publish only from the repository's guarded `publish/v2.0.3` flow.

No tag is created or moved until those gates pass.

## Success criteria

The work is complete when:

- generated schema property selection matches default `encoding/json` for the specified embedded/dominance fixtures;
- `json:",string"` produces a correct string wire schema for supported primitive fields without invalid numeric/boolean schema keywords;
- JSON-compatible integer and `TextMarshaler` map keys compile successfully while unsupported keys still fail;
- hidden/ambiguous fields do not leak into properties or required lists and do not trigger schema errors;
- no exported oashttp API or runtime dependency is added;
- full CI and public API compatibility gates pass on the final merged commit;
- the patch release gate accepts the finalized release version before publication.