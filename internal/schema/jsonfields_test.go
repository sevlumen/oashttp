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

type JSONEmbedded struct {
	Promoted string `json:"promoted"`
}

type JSONEmbeddedPointer struct {
	PointerValue string `json:"pointerValue"`
}

type jsonPrivateEmbedded struct {
	PrivatePromoted string `json:"privatePromoted"`
}

type jsonNamedEmbedded struct {
	Value string `json:"value"`
}

type jsonLeft struct {
	Conflict string
}

type jsonRight struct {
	Conflict string
}

type jsonTaggedWinner struct {
	Value string `json:"Winner"`
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
	JSONEmbedded
	*JSONEmbeddedPointer
	jsonPrivateEmbedded
	jsonNamedEmbedded `json:"named"`
	Skip              string `json:"-"`
	Dash              string `json:"-,"`
	Invalid           string `json:"bad\\name"`
	Spaced            string `json:"space name"`
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

func TestSchemaFieldSelectionMatchesEncodingJSON(t *testing.T) {
	pointer := &JSONEmbeddedPointer{PointerValue: "p"}
	cases := []any{
		jsonSelectionFixture{
			JSONEmbedded:        JSONEmbedded{Promoted: "a"},
			JSONEmbeddedPointer: pointer,
			jsonPrivateEmbedded: jsonPrivateEmbedded{PrivatePromoted: "private"},
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
