package validation

import (
	"reflect"

	"github.com/quang020102/go-osm/internal/core"
)

func (p *Plan) validateReflect(v reflect.Value) []core.FieldError {
	for v.IsValid() && (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if !v.IsValid() || v.Kind() != reflect.Struct {
		return []core.FieldError{{Location: "body", Field: "", Messages: []string{"invalid validation value"}}}
	}
	var out []core.FieldError
	for _, field := range p.fields {
		fv, ok := fieldByIndex(v, field.index)
		if !ok {
			continue
		}
		var messages []string
		for _, r := range field.rules {
			if msg := validateRule(fv, r); msg != "" {
				messages = append(messages, msg)
			}
		}
		if len(messages) > 0 {
			out = append(out, core.FieldError{Location: "body", Field: field.name, Messages: messages})
		}
	}
	return out
}
func fieldByIndex(v reflect.Value, index []int) (reflect.Value, bool) {
	for _, i := range index {
		for v.Kind() == reflect.Pointer {
			if v.IsNil() {
				return v, false
			}
			v = v.Elem()
		}
		if v.Kind() != reflect.Struct || i >= v.NumField() {
			return reflect.Value{}, false
		}
		v = v.Field(i)
	}
	return v, true
}
