package validation

import (
	"encoding"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/quang020102/go-osm/internal/core"
)

type ruleKind uint8

const (
	ruleRequired ruleKind = iota
	ruleMin
	ruleMax
	ruleLen
	ruleEmail
	ruleUUID
	ruleE164
	ruleOneOf
	ruleGTE
	ruleLTE
)

type compiledRule struct {
	kind    ruleKind
	number  float64
	integer int
	choices []string
}
type fieldPlan struct {
	index []int
	name  string
	rules []compiledRule
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
			rules, err := compileRules(f.Type, tag)
			if err != nil {
				return fmt.Errorf("field %s.%s: %w", t, f.Name, err)
			}
			*out = append(*out, fieldPlan{index: idx, name: jsonName, rules: rules})
		}
		nested := f.Type
		for nested.Kind() == reflect.Pointer {
			nested = nested.Elem()
		}
		if nested.Kind() == reflect.Struct && nested.PkgPath() != "time" && !(nested.PkgPath() == "github.com/quang020102/go-osm" && nested.Name() == "UUID") && !nested.Implements(textUnmarshalerType) && !reflect.PointerTo(nested).Implements(textUnmarshalerType) {
			if err := compileStruct(nested, idx, out, nextStack); err != nil {
				return err
			}
		}
	}
	return nil
}
func compileRules(t reflect.Type, tag string) ([]compiledRule, error) {
	var out []compiledRule
	for _, raw := range strings.Split(tag, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		parts := strings.SplitN(raw, "=", 2)
		name := parts[0]
		arg := ""
		if len(parts) == 2 {
			arg = parts[1]
		}
		r := compiledRule{}
		switch name {
		case "required":
			r.kind = ruleRequired
			if arg != "" {
				return nil, fmt.Errorf("required takes no argument")
			}
		case "min", "max", "len":
			if arg == "" {
				return nil, fmt.Errorf("%s requires an argument", name)
			}
			n, err := strconv.Atoi(arg)
			if err != nil {
				return nil, fmt.Errorf("%s argument %q is invalid", name, arg)
			}
			if !supportsLengthOrNumber(t) {
				return nil, fmt.Errorf("%s is incompatible with %s", name, t)
			}
			r.integer = n
			if name == "min" {
				r.kind = ruleMin
			} else if name == "max" {
				r.kind = ruleMax
			} else {
				r.kind = ruleLen
			}
		case "email":
			if baseKind(t) != reflect.String {
				return nil, fmt.Errorf("email is incompatible with %s", t)
			}
			r.kind = ruleEmail
		case "uuid":
			if baseKind(t) != reflect.String {
				return nil, fmt.Errorf("uuid is incompatible with %s", t)
			}
			r.kind = ruleUUID
		case "e164":
			if baseKind(t) != reflect.String {
				return nil, fmt.Errorf("e164 is incompatible with %s", t)
			}
			r.kind = ruleE164
		case "oneof":
			if arg == "" {
				return nil, fmt.Errorf("oneof requires values")
			}
			r.kind = ruleOneOf
			r.choices = strings.Fields(arg)
		case "gte", "lte":
			if !isNumericKind(baseKind(t)) {
				return nil, fmt.Errorf("%s is incompatible with %s", name, t)
			}
			n, err := strconv.ParseFloat(arg, 64)
			if err != nil {
				return nil, fmt.Errorf("%s argument %q is invalid", name, arg)
			}
			r.number = n
			if name == "gte" {
				r.kind = ruleGTE
			} else {
				r.kind = ruleLTE
			}
		default:
			return nil, fmt.Errorf("unknown validation rule %q", name)
		}
		out = append(out, r)
	}
	return out, nil
}
func baseKind(t reflect.Type) reflect.Kind {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Kind()
}
func supportsLengthOrNumber(t reflect.Type) bool {
	k := baseKind(t)
	return k == reflect.String || k == reflect.Slice || k == reflect.Array || k == reflect.Map || isNumericKind(k)
}
func isNumericKind(k reflect.Kind) bool {
	return (k >= reflect.Int && k <= reflect.Float64) || (k >= reflect.Uint && k <= reflect.Uint64)
}

func (p *Plan) Validate(value any) []core.FieldError {
	return p.validateReflect(reflect.ValueOf(value))
}
