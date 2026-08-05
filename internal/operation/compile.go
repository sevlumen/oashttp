package operation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/quang020102/go-osm/internal/binding"
	"github.com/quang020102/go-osm/internal/core"
	"github.com/quang020102/go-osm/internal/oas31"
	"github.com/quang020102/go-osm/internal/route"
	"github.com/quang020102/go-osm/internal/schema"
	internalsecurity "github.com/quang020102/go-osm/internal/security"
	"github.com/quang020102/go-osm/internal/validation"
)

type Options struct {
	Binding       binding.Options
	Registry      *schema.Registry
	Validator     core.Validator
	Authenticator core.Authenticator
	Authorizer    core.Authorizer
	ErrorHandler  func(context.Context, error)
}
type Compiled struct {
	Pattern   route.Pattern
	Handler   http.Handler
	Operation *oas31.Operation
	Protected bool
}

func Compile(def *Definition, opts Options) (Compiled, error) {
	if strings.TrimSpace(def.OperationID) == "" {
		return Compiled{}, fmt.Errorf("%s %s (%s -> %s): operation ID is required", def.Method, def.UserRoute, def.InputType, def.OutputType)
	}
	if (def.Feature == "") != (def.Permission == "") {
		return Compiled{}, fmt.Errorf("%s %s (%s): feature and permission must be configured together", def.Method, def.UserRoute, def.InputType)
	}
	for status := range def.Responses {
		if status < 100 || status > 599 {
			return Compiled{}, fmt.Errorf("%s %s (%s -> %s): invalid response status %d", def.Method, def.UserRoute, def.InputType, def.OutputType, status)
		}
	}
	pattern, err := route.Parse(def.FullRoute)
	if err != nil {
		return Compiled{}, fmt.Errorf("%s %s (%s -> %s): %w", def.Method, def.UserRoute, def.InputType, def.OutputType, err)
	}
	bindPlan, err := binding.Compile(def.InputType, pattern, opts.Binding)
	if err != nil {
		return Compiled{}, fmt.Errorf("%s %s (%s -> %s): %w", def.Method, def.UserRoute, def.InputType, def.OutputType, err)
	}
	var validationPlan *validation.Plan
	if def.Validation {
		validationPlan, err = validation.Compile(def.InputType)
		if err != nil {
			return Compiled{}, fmt.Errorf("%s %s (%s): %w", def.Method, def.UserRoute, def.InputType, err)
		}
	}
	if def.Feature != "" && opts.Authenticator == nil {
		return Compiled{}, fmt.Errorf("%s %s (%s): protected operation requires Config.Authenticator", def.Method, def.UserRoute, def.InputType)
	}
	oasOperation, err := compileOAS(def, bindPlan, pattern, opts.Registry)
	if err != nil {
		return Compiled{}, fmt.Errorf("%s %s (%s -> %s): %w", def.Method, def.UserRoute, def.InputType, def.OutputType, err)
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if def.Feature != "" {
			next, status, code, detail := internalsecurity.AuthenticateAndAuthorize(ctx, r.Header.Get("Authorization"), def.Feature, def.Permission, opts.Authenticator, opts.Authorizer)
			if status != 0 {
				writeProblem(w, status, code, detail)
				return
			}
			ctx = next
			r = r.WithContext(ctx)
		}
		input, fieldErrors := bindPlan.Bind(r)
		if def.Validation {
			fieldErrors = append(fieldErrors, validationPlan.Validate(input.Interface())...)
			if opts.Validator != nil {
				fieldErrors = append(fieldErrors, opts.Validator.Validate(ctx, input.Interface())...)
			}
		}
		if len(fieldErrors) > 0 {
			writeValidationProblem(w, fieldErrors)
			return
		}
		result := def.Invoke(ctx, input)
		if result == nil {
			writeProblem(w, 500, "NIL_RESULT", "The operation returned no result")
			return
		}
		result.WriteHTTP(w, func(err error) {
			if opts.ErrorHandler != nil {
				opts.ErrorHandler(ctx, err)
			}
		})
	})
	return Compiled{Pattern: pattern, Handler: handler, Operation: oasOperation, Protected: def.Feature != ""}, nil
}

func compileOAS(def *Definition, plan *binding.Plan, pattern route.Pattern, registry *schema.Registry) (*oas31.Operation, error) {
	if registry == nil {
		return nil, fmt.Errorf("schema registry is nil")
	}
	if def.OperationID == "" {
		return nil, fmt.Errorf("operation ID is required")
	}
	success := false
	responses := map[string]oas31.Response{}
	for status, spec := range def.Responses {
		if status >= 200 && status < 300 {
			success = true
		}
		description := spec.Description
		if description == "" {
			description = http.StatusText(status)
		}
		if description == "" {
			description = "Response"
		}
		response := oas31.Response{Description: description}
		if spec.Kind == ResponseProblem {
			ref := oas31.Schema{"$ref": "#/components/schemas/ProblemDetails"}
			response.Content = map[string]oas31.MediaType{"application/problem+json": {Schema: &ref}}
		} else if status != http.StatusNoContent {
			ref, err := registry.Ref(def.OutputType)
			if err != nil {
				return nil, err
			}
			response.Content = map[string]oas31.MediaType{"application/json": {Schema: ref}}
		}
		responses[strconv.Itoa(status)] = response
	}
	if !success {
		return nil, fmt.Errorf("at least one successful response must be declared")
	}
	operation := &oas31.Operation{OperationID: def.OperationID, Tags: append([]string(nil), def.Tags...), Summary: def.Summary, Description: def.Description, Responses: responses}
	params, body := plan.Documentation()
	routeConstraints := map[string]route.Constraint{}
	for _, p := range pattern.Parameters {
		routeConstraints[p.Name] = route.Constraints[p.Constraint]
	}
	for _, p := range params {
		var ref *oas31.Schema
		if p.In == "path" {
			c := routeConstraints[p.Name]
			s := oas31.Schema{"type": c.JSONType}
			if c.Format != "" {
				s["format"] = c.Format
			}
			ref = &s
		} else {
			var err error
			ref, err = registry.Ref(p.Type)
			if err != nil {
				return nil, fmt.Errorf("parameter %s: %w", p.Name, err)
			}
		}
		operation.Parameters = append(operation.Parameters, oas31.Parameter{Name: p.Name, In: p.In, Required: p.Required, Schema: ref})
	}
	if body != nil {
		ref, err := registry.Ref(body.Type)
		if err != nil {
			return nil, fmt.Errorf("request body: %w", err)
		}
		operation.RequestBody = &oas31.RequestBody{Required: body.Required, Content: map[string]oas31.MediaType{"application/json": {Schema: ref}}}
	}
	if def.Feature != "" {
		operation.Security = []map[string][]string{{"bearerAuth": {}}}
		operation.Extensions = map[string]any{"x-feature": def.Feature, "x-permission": def.Permission}
	}
	return operation, nil
}
func writeValidationProblem(w http.ResponseWriter, errors []core.FieldError) {
	grouped := map[string][]string{}
	for _, field := range errors {
		key := strings.Trim(strings.TrimSpace(field.Location)+"."+strings.TrimSpace(field.Field), ".")
		if key == "" {
			key = "request"
		}
		grouped[key] = append(grouped[key], field.Messages...)
	}
	writeProblemObject(w, core.ProblemDetails{Title: "Bad Request", Status: 400, Code: "VALIDATION_FAILED", Detail: "The request is invalid", Errors: grouped})
}
func writeProblem(w http.ResponseWriter, status int, code, detail string) {
	title := http.StatusText(status)
	if title == "" {
		title = "Error"
	}
	writeProblemObject(w, core.ProblemDetails{Title: title, Status: status, Code: code, Detail: detail})
}
func writeProblemObject(w http.ResponseWriter, p core.ProblemDetails) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}
