package schema

import (
	"encoding/json"
	"reflect"
	"testing"
)

type namedStringKey string

type valueTextKey struct {
	Value string
}

func (key valueTextKey) MarshalText() ([]byte, error) {
	return []byte(key.Value), nil
}

type pointerTextKey struct {
	Value string
}

func (key *pointerTextKey) MarshalText() ([]byte, error) {
	return []byte(key.Value), nil
}

type plainStructKey struct {
	Value string
}

func TestRegistryMapKeyCompatibilityMatchesEncodingJSON(t *testing.T) {
	supported := []any{
		map[string]int{"a": 1},
		map[namedStringKey]int{"a": 1},
		map[int]int{1: 1},
		map[int64]int{1: 1},
		map[uint64]int{1: 1},
		map[uintptr]int{1: 1},
		map[valueTextKey]int{{Value: "a"}: 1},
	}
	for _, value := range supported {
		typ := reflect.TypeOf(value)
		if _, err := json.Marshal(value); err != nil {
			t.Fatalf("encoding/json rejected supported %s: %v", typ, err)
		}
		registry := NewRegistry()
		if _, err := registry.Ref(typ); err != nil {
			t.Fatalf("schema rejected %s: %v", typ, err)
		}
	}

	unsupported := []any{
		map[bool]int{true: 1},
		map[plainStructKey]int{{Value: "a"}: 1},
		map[pointerTextKey]int{{Value: "a"}: 1},
	}
	for _, value := range unsupported {
		typ := reflect.TypeOf(value)
		if _, err := json.Marshal(value); err == nil {
			t.Fatalf("encoding/json unexpectedly accepted %s", typ)
		}
		registry := NewRegistry()
		if _, err := registry.Ref(typ); err == nil {
			t.Fatalf("schema unexpectedly accepted %s", typ)
		}
	}
}
