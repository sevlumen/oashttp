package binding

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/oashttp/oashttp/internal/route"
)

type Options struct {
	JSONBodyLimit             int64
	DisallowUnknownJSONFields bool
}
type sourceKind uint8

const (
	sourcePath sourceKind = iota
	sourceQuery
	sourceHeader
	sourceBody
)

type fieldPlan struct {
	index      []int
	name       string
	source     sourceKind
	typ        reflect.Type
	constraint string
	required   bool
}
type Plan struct {
	typ     reflect.Type
	fields  []fieldPlan
	body    *fieldPlan
	options Options
	pattern route.Pattern
}

func Compile(inputType reflect.Type, pattern route.Pattern, options Options) (*Plan, error) {
	if options.JSONBodyLimit <= 0 {
		options.JSONBodyLimit = 1 << 20
	}
	if inputType.Kind() == reflect.Pointer {
		return nil, fmt.Errorf("input type %s must be a non-pointer struct", inputType)
	}
	if inputType.Kind() != reflect.Struct {
		return nil, fmt.Errorf("input type %s must be a struct", inputType)
	}
	p := &Plan{typ: inputType, options: options, pattern: pattern}
	routeParams := map[string]string{}
	for _, rp := range pattern.Parameters {
		routeParams[rp.Name] = rp.Constraint
	}
	boundPath := map[string]bool{}
	for i := 0; i < inputType.NumField(); i++ {
		f := inputType.Field(i)
		tags := []struct {
			name   string
			source sourceKind
		}{{"path", sourcePath}, {"query", sourceQuery}, {"header", sourceHeader}, {"body", sourceBody}}
		found := 0
		for _, tag := range tags {
			name, ok := f.Tag.Lookup(tag.name)
			if !ok {
				continue
			}
			found++
			if f.PkgPath != "" {
				return nil, fmt.Errorf("%s %s: unexported tagged field %s", pattern.UserPath, inputType, f.Name)
			}
			if name == "" {
				return nil, fmt.Errorf("%s %s: field %s has empty %s tag", pattern.UserPath, inputType, f.Name, tag.name)
			}
			fp := fieldPlan{index: f.Index, name: name, source: tag.source, typ: f.Type, required: hasRequiredRule(f.Tag.Get("validate"))}
			switch tag.source {
			case sourcePath:
				constraint, exists := routeParams[name]
				if !exists {
					return nil, fmt.Errorf("%s %s: path field %s refers to missing parameter %q", pattern.UserPath, inputType, f.Name, name)
				}
				if boundPath[name] {
					return nil, fmt.Errorf("%s %s: duplicate path binding for %q", pattern.UserPath, inputType, name)
				}
				boundPath[name] = true
				fp.constraint = constraint
				fp.required = true
			case sourceBody:
				if name != "json" {
					return nil, fmt.Errorf("%s %s: body tag must be body:\"json\"", pattern.UserPath, inputType)
				}
				if p.body != nil {
					return nil, fmt.Errorf("%s %s: multiple body fields", pattern.UserPath, inputType)
				}
				if !supportedJSONType(f.Type) {
					return nil, fmt.Errorf("%s %s: unsupported JSON body type %s", pattern.UserPath, inputType, f.Type)
				}
				fp.required = f.Type.Kind() != reflect.Pointer
				p.body = &fp
			}
			p.fields = append(p.fields, fp)
		}
		if found > 1 {
			return nil, fmt.Errorf("%s %s: field %s has multiple binding tags", pattern.UserPath, inputType, f.Name)
		}
	}
	var missing []string
	for name := range routeParams {
		if !boundPath[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("%s %s: missing path field for %s", pattern.UserPath, inputType, strings.Join(missing, ", "))
	}
	return p, nil
}
func supportedJSONType(t reflect.Type) bool {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct, reflect.Map, reflect.Slice, reflect.Array, reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

type ParameterDoc struct {
	Name       string
	In         string
	Required   bool
	Type       reflect.Type
	Constraint string
}
type BodyDoc struct {
	Type     reflect.Type
	Required bool
}

func (p *Plan) Documentation() ([]ParameterDoc, *BodyDoc) {
	params := make([]ParameterDoc, 0, len(p.fields))
	var body *BodyDoc
	for _, f := range p.fields {
		switch f.source {
		case sourcePath:
			params = append(params, ParameterDoc{Name: f.name, In: "path", Required: true, Type: f.typ, Constraint: f.constraint})
		case sourceQuery:
			params = append(params, ParameterDoc{Name: f.name, In: "query", Required: f.required, Type: f.typ})
		case sourceHeader:
			params = append(params, ParameterDoc{Name: f.name, In: "header", Required: f.required, Type: f.typ})
		case sourceBody:
			body = &BodyDoc{Type: f.typ, Required: f.required}
		}
	}
	return params, body
}

func hasRequiredRule(tag string) bool {
	for _, part := range strings.Split(tag, ",") {
		if strings.TrimSpace(part) == "required" {
			return true
		}
	}
	return false
}
