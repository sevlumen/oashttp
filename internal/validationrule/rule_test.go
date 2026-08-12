package validationrule

import (
	"reflect"
	"testing"
)

func TestParseSupportedRules(t *testing.T) {
	rules, err := Parse(reflect.TypeOf(""), "required,min=2,max=10,len=4,email,uuid,e164,oneof=admin user")
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 8 {
		t.Fatalf("rules=%#v", rules)
	}
	if rules[0].Kind != Required || rules[1].Kind != Min || rules[1].Integer != 2 || rules[7].Kind != OneOf {
		t.Fatalf("rules=%#v", rules)
	}
	if !reflect.DeepEqual(rules[7].Choices, []string{"admin", "user"}) {
		t.Fatalf("choices=%#v", rules[7].Choices)
	}
}

func TestParseNumericBoundsAndTypedOneOf(t *testing.T) {
	for _, tc := range []struct {
		typ reflect.Type
		tag string
	}{
		{reflect.TypeOf(int(0)), "min=1,max=9,len=5,gte=1,lte=9,oneof=1 5 9"},
		{reflect.TypeOf(uint(0)), "oneof=4 8"},
		{reflect.TypeOf(float64(0)), "oneof=0.5 1.5"},
		{reflect.TypeOf(false), "oneof=true false"},
	} {
		if _, err := Parse(tc.typ, tc.tag); err != nil {
			t.Fatalf("Parse(%s, %q): %v", tc.typ, tc.tag, err)
		}
	}
}

func TestParseRejectsInvalidContracts(t *testing.T) {
	for _, tc := range []struct {
		typ reflect.Type
		tag string
	}{
		{reflect.TypeOf(""), "unknown"},
		{reflect.TypeOf(""), "required=yes"},
		{reflect.TypeOf(""), "min"},
		{reflect.TypeOf(1), "email"},
		{reflect.TypeOf([]string{}), "oneof=a b"},
		{reflect.TypeOf(1), "oneof=x y"},
		{reflect.TypeOf(false), "oneof=yes no"},
	} {
		if _, err := Parse(tc.typ, tc.tag); err == nil {
			t.Fatalf("Parse(%s, %q) succeeded", tc.typ, tc.tag)
		}
	}
}

func TestScalarValuePreservesJSONScalarTypes(t *testing.T) {
	for _, tc := range []struct {
		typ  reflect.Type
		text string
		want any
	}{
		{reflect.TypeOf(""), "admin", "admin"},
		{reflect.TypeOf(false), "true", true},
		{reflect.TypeOf(int(0)), "5", int64(5)},
		{reflect.TypeOf(uint(0)), "8", uint64(8)},
		{reflect.TypeOf(float64(0)), "1.5", float64(1.5)},
	} {
		got, err := ScalarValue(tc.typ, tc.text)
		if err != nil {
			t.Fatalf("ScalarValue(%s, %q): %v", tc.typ, tc.text, err)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("ScalarValue(%s, %q)=%#v want=%#v", tc.typ, tc.text, got, tc.want)
		}
	}
}
