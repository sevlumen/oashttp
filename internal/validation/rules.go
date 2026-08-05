package validation

import (
	"fmt"
	"net/mail"
	"reflect"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/oashttp/oashttp/internal/core"
)

var e164Pattern = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)

func validateRule(v reflect.Value, r compiledRule) string {
	switch r.kind {
	case ruleRequired:
		if isZero(v) {
			return "is required"
		}
	}
	if isNil(v) {
		return ""
	}
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		v = v.Elem()
	}
	switch r.kind {
	case ruleMin:
		if measure(v) < float64(r.integer) {
			return fmt.Sprintf("must be at least %d", r.integer)
		}
	case ruleMax:
		if measure(v) > float64(r.integer) {
			return fmt.Sprintf("must be at most %d", r.integer)
		}
	case ruleLen:
		if measure(v) != float64(r.integer) {
			return fmt.Sprintf("must have length %d", r.integer)
		}
	case ruleEmail:
		s := strings.TrimSpace(v.String())
		a, err := mail.ParseAddress(s)
		if err != nil || a.Address != s {
			return "must be a valid email address"
		}
	case ruleUUID:
		if _, err := core.NormalizeUUID(v.String()); err != nil {
			return "must be a valid UUID"
		}
	case ruleE164:
		if !e164Pattern.MatchString(v.String()) {
			return "must be a valid E.164 phone number"
		}
	case ruleOneOf:
		actual := fmt.Sprint(v.Interface())
		for _, choice := range r.choices {
			if actual == choice {
				return ""
			}
		}
		return "must be one of " + strings.Join(r.choices, ", ")
	case ruleGTE:
		if numeric(v) < r.number {
			return fmt.Sprintf("must be greater than or equal to %v", r.number)
		}
	case ruleLTE:
		if numeric(v) > r.number {
			return fmt.Sprintf("must be less than or equal to %v", r.number)
		}
	}
	return ""
}
func isNil(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Slice, reflect.Map:
		return v.IsNil()
	}
	return false
}
func isZero(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}
	return v.IsZero()
}
func measure(v reflect.Value) float64 {
	switch v.Kind() {
	case reflect.String:
		return float64(utf8.RuneCountInString(v.String()))
	case reflect.Slice, reflect.Array, reflect.Map:
		return float64(v.Len())
	default:
		return numeric(v)
	}
}
func numeric(v reflect.Value) float64 {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(v.Uint())
	case reflect.Float32, reflect.Float64:
		return v.Float()
	}
	return 0
}
