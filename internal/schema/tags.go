package schema

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"

	"github.com/sevlumen/oashttp/v2/internal/validationrule"
)

func applyFieldTags(schema map[string]any, field reflect.StructField, rules []validationrule.Rule) error {
	if v := field.Tag.Get("description"); v != "" {
		schema["description"] = v
	}
	if v := field.Tag.Get("format"); v != "" {
		schema["format"] = v
	}
	if v := field.Tag.Get("example"); v != "" {
		schema["example"] = parseExample(v, field.Type)
	}

	for _, rule := range rules {
		switch rule.Kind {
		case validationrule.Required:
			continue
		case validationrule.Email:
			schema["format"] = "email"
		case validationrule.UUID:
			schema["format"] = "uuid"
		case validationrule.E164:
			continue
		case validationrule.OneOf:
			values := make([]any, 0, len(rule.Choices))
			for _, choice := range rule.Choices {
				value, err := validationrule.ScalarValue(field.Type, choice)
				if err != nil {
					return fmt.Errorf("oneof value %q: %w", choice, err)
				}
				values = append(values, value)
			}
			schema["enum"] = values
		case validationrule.Min:
			applyLengthOrNumberBound(schema, field.Type, rule.Integer, true)
		case validationrule.Max:
			applyLengthOrNumberBound(schema, field.Type, rule.Integer, false)
		case validationrule.Len:
			applyExactLengthOrNumber(schema, field.Type, rule.Integer)
		case validationrule.GTE:
			schema["minimum"] = rule.Number
		case validationrule.LTE:
			schema["maximum"] = rule.Number
		}
	}
	return nil
}

func applyQuotedJSONSchema(schema map[string]any, field reflect.StructField) error {
	var description any
	if value, ok := schema["description"]; ok {
		description = value
	}

	var example any
	if value, ok := schema["example"]; ok {
		lexical, err := quotedJSONLexical(value)
		if err != nil {
			return fmt.Errorf("quoted example: %w", err)
		}
		example = lexical
	}

	var enum []any
	if values, ok := schema["enum"].([]any); ok {
		enum = make([]any, 0, len(values))
		for _, value := range values {
			lexical, err := quotedJSONLexical(value)
			if err != nil {
				return fmt.Errorf("quoted enum: %w", err)
			}
			enum = append(enum, lexical)
		}
	}

	for key := range schema {
		delete(schema, key)
	}

	stringSchema := map[string]any{"type": "string"}
	if format := field.Tag.Get("format"); format != "" {
		stringSchema["format"] = format
	}
	if enum != nil {
		stringSchema["enum"] = enum
	}

	if field.Type.Kind() == reflect.Pointer {
		schema["oneOf"] = []any{stringSchema, map[string]any{"type": "null"}}
	} else {
		for key, value := range stringSchema {
			schema[key] = value
		}
	}
	if description != nil {
		schema["description"] = description
	}
	if example != nil {
		schema["example"] = example
	}
	return nil
}

func quotedJSONLexical(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func applyLengthOrNumberBound(schema map[string]any, t reflect.Type, n int, min bool) {
	kind := validationrule.BaseKind(t)
	switch kind {
	case reflect.String:
		if min {
			schema["minLength"] = n
		} else {
			schema["maxLength"] = n
		}
	case reflect.Slice, reflect.Array:
		if min {
			schema["minItems"] = n
		} else {
			schema["maxItems"] = n
		}
	case reflect.Map:
		if min {
			schema["minProperties"] = n
		} else {
			schema["maxProperties"] = n
		}
	default:
		if min {
			schema["minimum"] = float64(n)
		} else {
			schema["maximum"] = float64(n)
		}
	}
}

func applyExactLengthOrNumber(schema map[string]any, t reflect.Type, n int) {
	kind := validationrule.BaseKind(t)
	switch kind {
	case reflect.String:
		schema["minLength"] = n
		schema["maxLength"] = n
	case reflect.Slice, reflect.Array:
		schema["minItems"] = n
		schema["maxItems"] = n
	case reflect.Map:
		schema["minProperties"] = n
		schema["maxProperties"] = n
	default:
		schema["minimum"] = float64(n)
		schema["maximum"] = float64(n)
	}
}

func hasRuleKind(rules []validationrule.Rule, kind validationrule.Kind) bool {
	for _, rule := range rules {
		if rule.Kind == kind {
			return true
		}
	}
	return false
}

func parseExample(v string, t reflect.Type) any {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Bool:
		if x, e := strconv.ParseBool(v); e == nil {
			return x
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if x, e := strconv.ParseInt(v, 10, 64); e == nil {
			return x
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if x, e := strconv.ParseUint(v, 10, 64); e == nil {
			return x
		}
	case reflect.Float32, reflect.Float64:
		if x, e := strconv.ParseFloat(v, 64); e == nil {
			return x
		}
	}
	return v
}
