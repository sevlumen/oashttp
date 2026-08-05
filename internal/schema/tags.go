package schema

import (
	"reflect"
	"strconv"
	"strings"
)

func parseJSONTag(field reflect.StructField) (string, bool, bool) {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "", false, true
	}
	parts := strings.Split(tag, ",")
	name := field.Name
	if len(parts) > 0 && parts[0] != "" {
		name = parts[0]
	}
	omit := false
	for _, p := range parts[1:] {
		if p == "omitempty" {
			omit = true
		}
	}
	return name, omit, false
}
func hasRule(tag, name string) bool {
	for _, part := range strings.Split(tag, ",") {
		if part == name || strings.HasPrefix(part, name+"=") {
			return true
		}
	}
	return false
}
func applyFieldTags(schema map[string]any, field reflect.StructField) {
	if v := field.Tag.Get("description"); v != "" {
		schema["description"] = v
	}
	if v := field.Tag.Get("format"); v != "" {
		schema["format"] = v
	}
	if v := field.Tag.Get("example"); v != "" {
		schema["example"] = parseExample(v, field.Type)
	}
	for _, rule := range strings.Split(field.Tag.Get("validate"), ",") {
		if rule == "" || rule == "required" {
			continue
		}
		parts := strings.SplitN(rule, "=", 2)
		name := parts[0]
		arg := ""
		if len(parts) == 2 {
			arg = parts[1]
		}
		switch name {
		case "email":
			schema["format"] = "email"
		case "uuid":
			schema["format"] = "uuid"
		case "oneof":
			schema["enum"] = strings.Fields(arg)
		case "min":
			applyBound(schema, field.Type, arg, true)
		case "max":
			applyBound(schema, field.Type, arg, false)
		case "len":
			if n, err := strconv.Atoi(arg); err == nil {
				if baseKind(field.Type) == reflect.String {
					schema["minLength"] = n
					schema["maxLength"] = n
				} else {
					schema["minItems"] = n
					schema["maxItems"] = n
				}
			}
		case "gte":
			if n, err := strconv.ParseFloat(arg, 64); err == nil {
				schema["minimum"] = n
			}
		case "lte":
			if n, err := strconv.ParseFloat(arg, 64); err == nil {
				schema["maximum"] = n
			}
		}
	}
}
func applyBound(schema map[string]any, t reflect.Type, arg string, min bool) {
	n, err := strconv.ParseFloat(arg, 64)
	if err != nil {
		return
	}
	kind := baseKind(t)
	switch kind {
	case reflect.String:
		if min {
			schema["minLength"] = int(n)
		} else {
			schema["maxLength"] = int(n)
		}
	case reflect.Slice, reflect.Array, reflect.Map:
		if min {
			schema["minItems"] = int(n)
		} else {
			schema["maxItems"] = int(n)
		}
	default:
		if min {
			schema["minimum"] = n
		} else {
			schema["maximum"] = n
		}
	}
}
func baseKind(t reflect.Type) reflect.Kind {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Kind()
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
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
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
