package validationrule

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

type Kind uint8

const (
	Required Kind = iota
	Min
	Max
	Len
	Email
	UUID
	E164
	OneOf
	GTE
	LTE
)

type Rule struct {
	Kind    Kind
	Integer int
	Number  float64
	Choices []string
}

func Parse(t reflect.Type, tag string) ([]Rule, error) {
	var out []Rule
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

		rule := Rule{}
		switch name {
		case "required":
			if arg != "" {
				return nil, fmt.Errorf("required takes no argument")
			}
			rule.Kind = Required
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
			rule.Integer = n
			switch name {
			case "min":
				rule.Kind = Min
			case "max":
				rule.Kind = Max
			default:
				rule.Kind = Len
			}
		case "email", "uuid", "e164":
			if BaseKind(t) != reflect.String {
				return nil, fmt.Errorf("%s is incompatible with %s", name, t)
			}
			switch name {
			case "email":
				rule.Kind = Email
			case "uuid":
				rule.Kind = UUID
			default:
				rule.Kind = E164
			}
		case "oneof":
			if arg == "" {
				return nil, fmt.Errorf("oneof requires values")
			}
			if !isScalarKind(BaseKind(t)) {
				return nil, fmt.Errorf("oneof is incompatible with %s", t)
			}
			choices := strings.Fields(arg)
			if len(choices) == 0 {
				return nil, fmt.Errorf("oneof requires values")
			}
			for _, choice := range choices {
				if _, err := ScalarValue(t, choice); err != nil {
					return nil, fmt.Errorf("oneof value %q is invalid for %s: %w", choice, t, err)
				}
			}
			rule.Kind = OneOf
			rule.Choices = choices
		case "gte", "lte":
			if !IsNumericKind(BaseKind(t)) {
				return nil, fmt.Errorf("%s is incompatible with %s", name, t)
			}
			n, err := strconv.ParseFloat(arg, 64)
			if err != nil {
				return nil, fmt.Errorf("%s argument %q is invalid", name, arg)
			}
			rule.Number = n
			if name == "gte" {
				rule.Kind = GTE
			} else {
				rule.Kind = LTE
			}
		default:
			return nil, fmt.Errorf("unknown validation rule %q", name)
		}
		out = append(out, rule)
	}
	return out, nil
}

func BaseKind(t reflect.Type) reflect.Kind {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Kind()
}

func IsNumericKind(k reflect.Kind) bool {
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

func ScalarValue(t reflect.Type, text string) (any, error) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.String:
		return text, nil
	case reflect.Bool:
		value, err := strconv.ParseBool(text)
		if err != nil || strconv.FormatBool(value) != text {
			return nil, fmt.Errorf("expected canonical boolean")
		}
		return value, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value, err := strconv.ParseInt(text, 10, t.Bits())
		if err != nil || strconv.FormatInt(value, 10) != text {
			return nil, fmt.Errorf("expected canonical signed integer")
		}
		return value, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		value, err := strconv.ParseUint(text, 10, t.Bits())
		if err != nil || strconv.FormatUint(value, 10) != text {
			return nil, fmt.Errorf("expected canonical unsigned integer")
		}
		return value, nil
	case reflect.Float32, reflect.Float64:
		value, err := strconv.ParseFloat(text, t.Bits())
		if err != nil || strconv.FormatFloat(value, 'g', -1, t.Bits()) != text {
			return nil, fmt.Errorf("expected canonical floating-point number")
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported scalar type %s", t)
	}
}

func supportsLengthOrNumber(t reflect.Type) bool {
	kind := BaseKind(t)
	return kind == reflect.String || kind == reflect.Slice || kind == reflect.Array || kind == reflect.Map || IsNumericKind(kind)
}

func isScalarKind(k reflect.Kind) bool {
	return k == reflect.String || k == reflect.Bool || IsNumericKind(k)
}
