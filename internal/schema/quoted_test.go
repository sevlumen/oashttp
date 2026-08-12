package schema

import (
	"encoding/json"
	"reflect"
	"testing"
)

type quotedSchemaFixture struct {
	Count   int64   `json:"count,string" validate:"gte=1,lte=9,oneof=1 2" example:"2"`
	Active  bool    `json:"active,string" validate:"oneof=true false"`
	Ratio   float64 `json:"ratio,string"`
	Text    string  `json:"text,string" validate:"oneof=admin user" example:"admin"`
	Maybe   *int64  `json:"maybe,string"`
	Address uintptr `json:"address,string"`
	Custom  int64   `json:"custom,string" format:"digits"`
	Plain   []int   `json:"plain,string"`
}

func TestQuotedJSONFieldsDescribeWireStrings(t *testing.T) {
	registry := NewRegistry()
	if _, err := registry.Ref(reflect.TypeOf(quotedSchemaFixture{})); err != nil {
		t.Fatal(err)
	}
	component := registry.Components()["quotedSchemaFixture"]
	if component == nil {
		t.Fatal("missing quotedSchemaFixture component")
	}
	properties := (*component)["properties"].(map[string]any)

	count := properties["count"].(map[string]any)
	assertStringSchema(t, "count", count)
	for _, forbidden := range []string{"minimum", "maximum"} {
		if _, ok := count[forbidden]; ok {
			t.Fatalf("count retains %s: %#v", forbidden, count)
		}
	}
	assertStringEnum(t, "count", count["enum"], wireLexical(t, int64(1)), wireLexical(t, int64(2)))
	if got := count["example"]; got != wireLexical(t, int64(2)) {
		t.Fatalf("count example=%#v", got)
	}

	active := properties["active"].(map[string]any)
	assertStringSchema(t, "active", active)
	assertStringEnum(t, "active", active["enum"], wireLexical(t, true), wireLexical(t, false))

	ratio := properties["ratio"].(map[string]any)
	assertStringSchema(t, "ratio", ratio)

	text := properties["text"].(map[string]any)
	assertStringSchema(t, "text", text)
	assertStringEnum(t, "text", text["enum"], wireLexical(t, "admin"), wireLexical(t, "user"))
	if got := text["example"]; got != wireLexical(t, "admin") {
		t.Fatalf("text example=%#v", got)
	}

	maybe := properties["maybe"].(map[string]any)
	branches, ok := maybe["oneOf"].([]any)
	if !ok || len(branches) != 2 {
		t.Fatalf("maybe=%#v", maybe)
	}
	stringBranch, ok := branches[0].(map[string]any)
	if !ok || stringBranch["type"] != "string" {
		t.Fatalf("maybe string branch=%#v", branches[0])
	}
	nullBranch, ok := branches[1].(map[string]any)
	if !ok || nullBranch["type"] != "null" {
		t.Fatalf("maybe null branch=%#v", branches[1])
	}

	address := properties["address"].(map[string]any)
	assertStringSchema(t, "address", address)

	custom := properties["custom"].(map[string]any)
	if custom["type"] != "string" || custom["format"] != "digits" {
		t.Fatalf("custom=%#v", custom)
	}

	plain := properties["plain"].(map[string]any)
	if plain["type"] != "array" {
		t.Fatalf("plain=%#v", plain)
	}
}

func assertStringSchema(t *testing.T, name string, schema map[string]any) {
	t.Helper()
	if schema["type"] != "string" {
		t.Fatalf("%s=%#v", name, schema)
	}
	if format, ok := schema["format"]; ok && format != nil {
		t.Fatalf("%s retains underlying format: %#v", name, schema)
	}
}

func assertStringEnum(t *testing.T, name string, value any, want ...string) {
	t.Helper()
	items, ok := value.([]any)
	if !ok {
		t.Fatalf("%s enum=%#v", name, value)
	}
	if len(items) != len(want) {
		t.Fatalf("%s enum=%#v want=%v", name, value, want)
	}
	for i, expected := range want {
		if got, ok := items[i].(string); !ok || got != expected {
			t.Fatalf("%s enum[%d]=%#v want=%q", name, i, items[i], expected)
		}
	}
}

func wireLexical(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
