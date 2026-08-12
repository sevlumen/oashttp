package validation

import (
	"encoding"
	"fmt"
	"reflect"
	"strings"

	"github.com/sevlumen/oashttp/v2/internal/core"
	"github.com/sevlumen/oashttp/v2/internal/validationrule"
)

type fieldPlan struct {
	index []int
	name  string
	rules []validationrule.Rule
}

var textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()

type Plan struct {
	typ    reflect.Type
	fields []fieldPlan
}

func Compile(t reflect.Type) (*Plan, error) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("validation type %s must be a struct", t)
	}
	plan := &Plan{typ: t}
	if err := compileStruct(t, nil, &plan.fields, map[reflect.Type]bool{}); err != nil {
		return nil, err
	}
	return plan, nil
}

func compileStruct(t reflect.Type, prefix []int, out *[]fieldPlan, stack map[reflect.Type]bool) error {
	if stack[t] {
		return nil
	}
	nextStack := make(map[reflect.Type]bool, len(stack)+1)
	for key, value := range stack {
		nextStack[key] = value
	}
	nextStack[t] = true
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		idx := append(append([]int{}, prefix...), i)
		tag := f.Tag.Get("validate")
		jsonName := f.Name
		if jt := f.Tag.Get("json"); jt != "" {
			p := strings.Split(jt, ",")[0]
			if p == "-" {
				continue
			}
			if p != "" {
				jsonName = p
			}
		}
		if tag != "" {
			rules, err := validationrule.Parse(f.Type, tag)
			if err != nil {
				return fmt.Errorf("field %s.%s: %w", t, f.Name, err)
			}
			*out = append(*out, fieldPlan{index: idx, name: jsonName, rules: rules})
		}
		nested := f.Type
		for nested.Kind() == reflect.Pointer {
			nested = nested.Elem()
		}
		if nested.Kind() == reflect.Struct && nested.PkgPath() != "time" && !(nested.PkgPath() == "github.com/sevlumen/oashttp/v2" && nested.Name() == "UUID") && !nested.Implements(textUnmarshalerType) && !reflect.PointerTo(nested).Implements(textUnmarshalerType) {
			if err := compileStruct(nested, idx, out, nextStack); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *Plan) Validate(value any) []core.FieldError {
	return p.validateReflect(reflect.ValueOf(value))
}
