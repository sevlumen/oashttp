package schema

import (
	"encoding"
	"fmt"
	"reflect"
	"time"

	"github.com/sevlumen/oashttp/v2/internal/oas31"
	"github.com/sevlumen/oashttp/v2/internal/validationrule"
)

type SchemaProvider interface{ JSONSchema() map[string]any }

type Registry struct {
	components map[string]*oas31.Schema
	names      map[reflect.Type]string
	used       map[string]reflect.Type
}

func NewRegistry() *Registry {
	return &Registry{components: map[string]*oas31.Schema{}, names: map[reflect.Type]string{}, used: map[string]reflect.Type{}}
}
func (r *Registry) Components() map[string]*oas31.Schema { return r.components }

var timeType = reflect.TypeOf(time.Time{})
var textMarshalerType = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
var schemaProviderType = reflect.TypeOf((*SchemaProvider)(nil)).Elem()

func (r *Registry) Ref(t reflect.Type) (*oas31.Schema, error) {
	return r.schemaFor(t, map[reflect.Type]bool{})
}

func (r *Registry) schemaFor(t reflect.Type, stack map[reflect.Type]bool) (*oas31.Schema, error) {
	if t == nil {
		s := oas31.Schema{"type": "null"}
		return &s, nil
	}
	if t.Kind() == reflect.Pointer {
		underlying, err := r.schemaFor(t.Elem(), stack)
		if err != nil {
			return nil, err
		}
		s := oas31.Schema{"oneOf": []any{map[string]any(*underlying), map[string]any{"type": "null"}}}
		return &s, nil
	}
	if provided, ok, err := providedSchema(t); ok || err != nil {
		return provided, err
	}
	if t == timeType {
		s := oas31.Schema{"type": "string", "format": "date-time"}
		return &s, nil
	}
	if t.PkgPath() == "github.com/sevlumen/oashttp/v2" && t.Name() == "UUID" {
		s := oas31.Schema{"type": "string", "format": "uuid"}
		return &s, nil
	}
	if t.Implements(textMarshalerType) || reflect.PointerTo(t).Implements(textMarshalerType) {
		s := oas31.Schema{"type": "string"}
		return &s, nil
	}
	switch t.Kind() {
	case reflect.String:
		s := oas31.Schema{"type": "string"}
		return &s, nil
	case reflect.Bool:
		s := oas31.Schema{"type": "boolean"}
		return &s, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		s := oas31.Schema{"type": "integer", "format": "int32"}
		return &s, nil
	case reflect.Int64:
		s := oas31.Schema{"type": "integer", "format": "int64"}
		return &s, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		s := oas31.Schema{"type": "integer", "format": "int32", "minimum": 0}
		return &s, nil
	case reflect.Uint64:
		s := oas31.Schema{"type": "integer", "format": "int64", "minimum": 0}
		return &s, nil
	case reflect.Float32:
		s := oas31.Schema{"type": "number", "format": "float"}
		return &s, nil
	case reflect.Float64:
		s := oas31.Schema{"type": "number", "format": "double"}
		return &s, nil
	case reflect.Slice, reflect.Array:
		item, err := r.schemaFor(t.Elem(), stack)
		if err != nil {
			return nil, err
		}
		s := oas31.Schema{"type": "array", "items": map[string]any(*item)}
		if t.Kind() == reflect.Array {
			s["minItems"] = t.Len()
			s["maxItems"] = t.Len()
		}
		return &s, nil
	case reflect.Map:
		if !supportsJSONMapKey(t.Key()) {
			return nil, fmt.Errorf("unsupported map key type %s in %s", t.Key(), t)
		}
		value, err := r.schemaFor(t.Elem(), stack)
		if err != nil {
			return nil, err
		}
		s := oas31.Schema{"type": "object", "additionalProperties": map[string]any(*value)}
		return &s, nil
	case reflect.Struct:
		if t.Name() != "" {
			name := r.componentName(t)
			ref := oas31.Schema{"$ref": "#/components/schemas/" + name}
			if _, ok := r.components[name]; ok {
				return &ref, nil
			}
			placeholder := oas31.Schema{}
			r.components[name] = &placeholder
			object, err := r.structSchema(t, stack)
			if err != nil {
				delete(r.components, name)
				return nil, err
			}
			r.components[name] = object
			return &ref, nil
		}
		return r.structSchema(t, stack)
	default:
		return nil, fmt.Errorf("unsupported Go type %s", t)
	}
}

func supportsJSONMapKey(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return true
	default:
		return t.Implements(textMarshalerType)
	}
}

func (r *Registry) structSchema(t reflect.Type, stack map[reflect.Type]bool) (*oas31.Schema, error) {
	if stack[t] {
		return nil, fmt.Errorf("recursive anonymous struct %s is unsupported", t)
	}
	next := make(map[reflect.Type]bool, len(stack)+1)
	for k, v := range stack {
		next[k] = v
	}
	next[t] = true

	properties := map[string]any{}
	required := []string{}
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
		if selected.Quoted {
			if err := applyQuotedJSONSchema(schemaMap, field); err != nil {
				return nil, fmt.Errorf("field %s.%s: %w", t, field.Name, err)
			}
		}
		properties[selected.Name] = schemaMap
		if hasRuleKind(rules, validationrule.Required) {
			required = append(required, selected.Name)
		}
	}

	out := oas31.Schema{"type": "object", "properties": properties}
	if len(required) > 0 {
		out["required"] = required
	}
	return &out, nil
}

func providedSchema(t reflect.Type) (*oas31.Schema, bool, error) {
	var value reflect.Value
	switch {
	case t.Implements(schemaProviderType):
		value = reflect.New(t).Elem()
	case t.Kind() != reflect.Pointer && reflect.PointerTo(t).Implements(schemaProviderType):
		value = reflect.New(t)
	default:
		return nil, false, nil
	}
	provider, ok := value.Interface().(SchemaProvider)
	if !ok {
		return nil, false, nil
	}
	raw := provider.JSONSchema()
	if raw == nil {
		return nil, true, fmt.Errorf("schema provider %s returned nil", t)
	}
	copySchema := oas31.Schema{}
	for key, item := range raw {
		copySchema[key] = item
	}
	return &copySchema, true, nil
}
