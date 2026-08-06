package binding

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"

	"github.com/sevlumen/oashttp/v2/internal/core"
	"github.com/sevlumen/oashttp/v2/internal/route"
)

func (p *Plan) Bind(r *http.Request) (reflect.Value, *RequestError, []core.FieldError) {
	value := reflect.New(p.typ).Elem()
	var errs []core.FieldError
	constraints := map[string]route.Constraint{}
	for _, rp := range p.pattern.Parameters {
		constraints[rp.Name] = route.Constraints[rp.Constraint]
	}
	for _, field := range p.fields {
		target := value.FieldByIndex(field.index)
		var values []string
		location := ""
		switch field.source {
		case sourcePath:
			location = "path"
			raw := r.PathValue(field.name)
			if raw == "" {
				errs = append(errs, fieldError(location, field.name, "is required"))
				continue
			}
			if c := constraints[field.name]; c.Validate != nil {
				if err := c.Validate(raw); err != nil {
					errs = append(errs, fieldError(location, field.name, fmt.Sprintf("does not satisfy %s constraint", field.constraint)))
					continue
				}
			}
			values = []string{raw}
		case sourceQuery:
			location = "query"
			values = r.URL.Query()[field.name]
		case sourceHeader:
			location = "header"
			values = r.Header.Values(field.name)
		case sourceBody:
			location = "body"
			if err := decodeBody(r, target, p.options.JSONBodyLimit, p.options.DisallowUnknownJSONFields, field.required); err != nil {
				var requestErr *RequestError
				if errors.As(err, &requestErr) {
					return value, requestErr, errs
				}
				errs = append(errs, fieldError(location, field.name, err.Error()))
			}
			continue
		}
		if len(values) == 0 {
			continue
		}
		var setErr error
		if field.source == sourcePath {
			setErr = setConstrainedValues(target, values, field.constraint)
		} else {
			setErr = setValues(target, values)
		}
		if setErr != nil {
			errs = append(errs, fieldError(location, field.name, setErr.Error()))
		}
	}
	return value, nil, errs
}

func fieldError(location, field, message string) core.FieldError {
	return core.FieldError{Location: location, Field: field, Messages: []string{message}}
}
