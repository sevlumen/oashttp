package validation

import (
	"reflect"
	"testing"
)

type Request struct {
	Name  string `json:"name" validate:"required,min=2,max=10"`
	Email string `json:"email" validate:"required,email"`
	Phone string `json:"phone" validate:"e164"`
	Role  string `json:"role" validate:"oneof=admin user"`
	Age   int    `json:"age" validate:"gte=18,lte=120"`
}

func TestCompiledValidation(t *testing.T) {
	p, e := Compile(reflect.TypeOf(Request{}))
	if e != nil {
		t.Fatal(e)
	}
	errs := p.Validate(Request{Name: "x", Email: "bad", Phone: "123", Role: "root", Age: 17})
	if len(errs) != 5 {
		t.Fatalf("errors=%#v", errs)
	}
}

type recursiveRequest struct {
	Name  string            `json:"name" validate:"required"`
	Child *recursiveRequest `json:"child,omitempty"`
}
type invalidRuleRequest struct {
	Value string `validate:"unknown"`
}

func TestCompileRecursiveAndInvalidRules(t *testing.T) {
	if _, err := Compile(reflect.TypeOf(recursiveRequest{})); err != nil {
		t.Fatal(err)
	}
	if _, err := Compile(reflect.TypeOf(invalidRuleRequest{})); err == nil {
		t.Fatal("expected invalid rule error")
	}
}

type scalarOneOfRequest struct {
	Level  int     `json:"level" validate:"oneof=1 2 3"`
	Count  uint    `json:"count" validate:"oneof=4 8"`
	Ratio  float64 `json:"ratio" validate:"oneof=0.5 1.5"`
	Active bool    `json:"active" validate:"oneof=true false"`
	Role   string  `json:"role" validate:"oneof=admin user"`
}

func TestScalarOneOfRuntimeSemanticsRemainCompatible(t *testing.T) {
	plan, err := Compile(reflect.TypeOf(scalarOneOfRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if errs := plan.Validate(scalarOneOfRequest{Level: 2, Count: 8, Ratio: 1.5, Active: true, Role: "admin"}); len(errs) != 0 {
		t.Fatalf("valid value errors=%#v", errs)
	}
	if errs := plan.Validate(scalarOneOfRequest{Level: 9, Count: 10, Ratio: 2.5, Active: true, Role: "root"}); len(errs) != 4 {
		t.Fatalf("invalid value errors=%#v", errs)
	}
}

type nonScalarOneOfRequest struct {
	Values []string `json:"values" validate:"oneof=a b"`
}

func TestCompileRejectsNonScalarOneOf(t *testing.T) {
	if _, err := Compile(reflect.TypeOf(nonScalarOneOfRequest{})); err == nil {
		t.Fatal("expected non-scalar oneof to be rejected")
	}
}
