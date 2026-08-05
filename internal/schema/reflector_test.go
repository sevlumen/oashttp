package schema

import (
	"reflect"
	"testing"
	"time"
)

type UpdateUserRequest struct {
	FullName    string  `json:"fullName" validate:"required,min=2,max=200"`
	Email       string  `json:"email" validate:"required,email"`
	PhoneNumber *string `json:"phoneNumber,omitempty" validate:"e164"`
}

func TestReflectStructSchema(t *testing.T) {
	registry := NewRegistry()
	ref, err := registry.Ref(reflect.TypeOf(UpdateUserRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if got := (*ref)["$ref"]; got != "#/components/schemas/UpdateUserRequest" {
		t.Fatalf("$ref=%#v", got)
	}
	component := registry.Components()["UpdateUserRequest"]
	req := (*component)["required"].([]string)
	if len(req) != 2 || req[0] != "fullName" || req[1] != "email" {
		t.Fatalf("required=%#v", req)
	}
}

type customSchema string

func (customSchema) JSONSchema() map[string]any {
	return map[string]any{"type": "string", "format": "custom"}
}
func TestSchemaProvider(t *testing.T) {
	registry := NewRegistry()
	schema, err := registry.Ref(reflect.TypeOf(customSchema("")))
	if err != nil {
		t.Fatal(err)
	}
	if (*schema)["format"] != "custom" {
		t.Fatalf("schema=%#v", *schema)
	}
}

type customText string

func (customText) MarshalText() ([]byte, error) { return []byte("custom"), nil }

type nilSchema string

func (nilSchema) JSONSchema() map[string]any { return nil }

type EmbeddedFields struct {
	Name string `json:"name" validate:"required"`
}

type schemaFixture struct {
	EmbeddedFields
	Enabled bool              `json:"enabled"`
	Count   int64             `json:"count"`
	Ratio   float32           `json:"ratio"`
	Tags    []string          `json:"tags"`
	Fixed   [2]int            `json:"fixed"`
	Lookup  map[string]uint64 `json:"lookup"`
	When    time.Time         `json:"when"`
	Text    customText        `json:"text"`
	Custom  customSchema      `json:"custom"`
	Maybe   *string           `json:"maybe"`
}

func TestRegistryCoversProductionSchemaKinds(t *testing.T) {
	registry := NewRegistry()
	ref, err := registry.Ref(reflect.TypeOf(schemaFixture{}))
	if err != nil {
		t.Fatal(err)
	}
	if (*ref)["$ref"] == nil {
		t.Fatalf("ref=%#v", *ref)
	}
	component := registry.Components()["schemaFixture"]
	if component == nil {
		t.Fatalf("components=%#v", registry.Components())
	}
	properties := (*component)["properties"].(map[string]any)
	checks := map[string]string{
		"enabled": "boolean",
		"count":   "integer",
		"ratio":   "number",
		"tags":    "array",
		"fixed":   "array",
		"lookup":  "object",
		"when":    "string",
		"text":    "string",
		"custom":  "string",
	}
	for name, expected := range checks {
		property := properties[name].(map[string]any)
		if property["type"] != expected {
			t.Fatalf("%s=%#v", name, property)
		}
	}
	maybe := properties["maybe"].(map[string]any)
	if _, ok := maybe["oneOf"]; !ok {
		t.Fatalf("maybe=%#v", maybe)
	}
	fixed := properties["fixed"].(map[string]any)
	if fixed["minItems"] != 2 || fixed["maxItems"] != 2 {
		t.Fatalf("fixed=%#v", fixed)
	}
	custom := properties["custom"].(map[string]any)
	if custom["format"] != "custom" {
		t.Fatalf("custom=%#v", custom)
	}
}

func TestRegistryRejectsUnsupportedTypesAndProviders(t *testing.T) {
	registry := NewRegistry()
	for _, typ := range []reflect.Type{
		reflect.TypeOf(map[int]string{}),
		reflect.TypeOf(make(chan int)),
		reflect.TypeOf(nilSchema("")),
	} {
		if _, err := registry.Ref(typ); err == nil {
			t.Fatalf("Ref(%s) succeeded", typ)
		}
	}
}

func TestRegistryHandlesNilTypeAndPrimitiveNumbers(t *testing.T) {
	registry := NewRegistry()
	nullSchema, err := registry.Ref(nil)
	if err != nil || (*nullSchema)["type"] != "null" {
		t.Fatalf("schema=%#v err=%v", nullSchema, err)
	}
	for _, tc := range []struct {
		value  any
		typeID string
		format string
	}{
		{value: uint(1), typeID: "integer", format: "int32"},
		{value: uint64(1), typeID: "integer", format: "int64"},
		{value: float64(1), typeID: "number", format: "double"},
	} {
		schemaValue, err := registry.Ref(reflect.TypeOf(tc.value))
		if err != nil {
			t.Fatal(err)
		}
		if (*schemaValue)["type"] != tc.typeID || (*schemaValue)["format"] != tc.format {
			t.Fatalf("schema=%#v", *schemaValue)
		}
	}
}
