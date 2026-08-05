package oas31

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestMarshalMinimalDocument(t *testing.T) {
	doc := Document{OpenAPI: "3.1.0", JSONSchemaDialect: "https://json-schema.org/draft/2020-12/schema", Info: Info{Title: "Users API", Version: "1.0.0"}, Servers: []Server{{URL: "/", Description: "Current server"}}, Paths: map[string]*PathItem{}, Components: Components{Schemas: map[string]*Schema{}}}
	data, err := Marshal(&doc)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(data) {
		t.Fatalf("invalid JSON: %s", data)
	}
	if !bytes.Contains(data, []byte(`"openapi":"3.1.0"`)) {
		t.Fatalf("missing version: %s", data)
	}
	if !bytes.Contains(data, []byte(`"url":"/"`)) {
		t.Fatalf("missing server: %s", data)
	}
}
func TestOperationExtensionsCloneAndDeterminism(t *testing.T) {
	doc := Document{OpenAPI: "3.1.0", Info: Info{Title: "x", Version: "1"}, Paths: map[string]*PathItem{"/x": {Get: &Operation{OperationID: "getX", Responses: map[string]Response{"200": {Description: "OK"}}, Extensions: map[string]any{"x-feature": "core", "x-permission": "read"}}}}}
	first, err := Marshal(&doc)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Marshal(&doc)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("marshal output is not deterministic")
	}
	if !bytes.Contains(first, []byte(`"x-feature":"core"`)) {
		t.Fatalf("extensions missing: %s", first)
	}
	clone, err := doc.Clone()
	if err != nil {
		t.Fatal(err)
	}
	clone.Info.Title = "changed"
	if doc.Info.Title == clone.Info.Title {
		t.Fatal("clone mutated original")
	}
}
