package schema

import (
	"reflect"
	"testing"
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
