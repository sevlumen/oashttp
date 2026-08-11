package operation

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/sevlumen/oashttp/v2/internal/binding"
	"github.com/sevlumen/oashttp/v2/internal/core"
	internalfailure "github.com/sevlumen/oashttp/v2/internal/failure"
	"github.com/sevlumen/oashttp/v2/internal/oas31"
	"github.com/sevlumen/oashttp/v2/internal/route"
	"github.com/sevlumen/oashttp/v2/internal/schema"
	internalsecurity "github.com/sevlumen/oashttp/v2/internal/security"
	"github.com/sevlumen/oashttp/v2/internal/validation"
)

type Options struct {
	Binding           binding.Options
	Registry          *schema.Registry
	Validator         core.Validator
	Authenticator     core.Authenticator
	Authorizer        core.Authorizer
	SecurityProviders map[string]core.SecurityProvider
	ErrorHandler      func(context.Context, error)

	FailureFormatter   core.FailureFormatter
	FailureContentType string
	FailureModelType   reflect.Type
}

type Compiled struct {
	Pattern      route.Pattern
	Handler      http.Handler
	Operation    *oas31.Operation
	Protected    bool
	SecurityName string
}

func Compile(def *Definition, opts Options) (Compiled, error) {
	if strings.TrimSpace(def.OperationID) == "" {
		return Compiled{}, fmt.Errorf("%s %s: operation ID is required", def.Method, def.UserRoute)
	}
	if (def.Feature == "") != (def.Permission == "") {
		return Compiled{}, fmt.Errorf("%s %s: feature and permission must be configured together", def.Method, def.UserRoute)
	}
	if def.SecurityName != "" {
		provider, ok := opts.SecurityProviders[def.SecurityName]
		if !ok || provider == nil {
			return Compiled{}, fmt.Errorf("%s %s: security provider %q is not configured", def.Method, def.UserRoute, def.SecurityName)
		}
	}
	if def.Feature != "" && def.SecurityName == "" && opts.Authenticator == nil {
		return Compiled{}, fmt.Errorf("%s %s: protected operation requires Config.Authenticator or RequireSecurity", def.Method, def.UserRoute)
	}
	for status, spec := range def.Responses {
		if status < 100 || status > 599 {
			return Compiled{}, fmt.Errorf("%s %s: invalid response status %d", def.Method, def.UserRoute, status)
		}
		if spec.Kind == ResponseCustom && statusAllowsResponseBody(status) {
			if spec.ModelType == nil {
				return Compiled{}, fmt.Errorf("%s %s: custom response %d requires a non-nil model", def.Method, def.UserRoute, status)
			}
			mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(spec.ContentType))
			if err != nil || mediaType == "" {
				return Compiled{}, fmt.Errorf("%s %s: custom response %d has invalid content type %q", def.Method, def.UserRoute, status, spec.ContentType)
			}
		}
	}
	for _, contentType := range def.RawRequestMediaTypes {
		mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(contentType))
		if err != nil || mediaType == "" {
			return Compiled{}, fmt.Errorf("%s %s: raw request has invalid content type %q", def.Method, def.UserRoute, contentType)
		}
	}

	pattern, err := route.Parse(def.FullRoute)
	if err != nil {
		return Compiled{}, fmt.Errorf("%s %s: %w", def.Method, def.UserRoute, err)
	}

	var bindPlan *binding.Plan
	var validationPlan *validation.Plan
	if def.RawHandler == nil {
		if def.InputType == nil || def.OutputType == nil || def.Invoke == nil {
			return Compiled{}, fmt.Errorf("%s %s: typed operation is incomplete", def.Method, def.UserRoute)
		}
		bindPlan, err = binding.Compile(def.InputType, pattern, opts.Binding)
		if err != nil {
			return Compiled{}, fmt.Errorf("%s %s (%s -> %s): %w", def.Method, def.UserRoute, def.InputType, def.OutputType, err)
		}
		if def.Validation {
			validationPlan, err = validation.Compile(def.InputType)
			if err != nil {
				return Compiled{}, fmt.Errorf("%s %s (%s): %w", def.Method, def.UserRoute, def.InputType, err)
			}
		}
	}

	if opts.FailureFormatter == nil {
		opts.FailureFormatter = core.ProblemDetailsFormatter{}
	}
	if opts.FailureContentType == "" || opts.FailureModelType == nil {
		opts.FailureContentType, opts.FailureModelType, err = internalfailure.Describe(opts.FailureFormatter)
		if err != nil {
			return Compiled{}, err
		}
	}

	oasOperation, err := compileOAS(def, bindPlan, pattern, opts.Registry, opts.FailureContentType, opts.FailureModelType)
	if err != nil {
		return Compiled{}, fmt.Errorf("%s %s: %w", def.Method, def.UserRoute, err)
	}

	var endpoint http.Handler
	if def.RawHandler != nil {
		endpoint = rawEndpoint(def.RawHandler, pattern, opts)
	} else {
		endpoint = typedEndpoint(def, bindPlan, validationPlan, opts)
	}
	for index := len(def.Middlewares) - 1; index >= 0; index-- {
		endpoint = def.Middlewares[index](endpoint)
	}

	effectiveSecurityName := def.SecurityName
	if effectiveSecurityName == "" && def.Feature != "" {
		effectiveSecurityName = "bearerAuth"
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := core.WithOperationInfo(r.Context(), core.OperationInfo{ID: def.OperationID, Method: def.Method, Route: pattern.OpenAPIPath})
		r = r.WithContext(ctx)

		report := func(err error) {
			if opts.ErrorHandler != nil {
				opts.ErrorHandler(ctx, err)
			}
		}
		writeFailure := func(item core.Failure) {
			internalfailure.WriteResolved(w, opts.FailureFormatter, opts.FailureContentType, item, report)
		}

		if def.SecurityName != "" {
			provider := opts.SecurityProviders[def.SecurityName]
			next, principal, securityFailure := internalsecurity.AuthenticateProvider(ctx, r, provider)
			if securityFailure != nil {
				applyProviderChallenge(w, provider, securityFailure)
				writeFailure(core.Failure{Status: securityFailure.Status, Code: securityFailure.Code, Detail: securityFailure.Detail})
				return
			}
			ctx = next
			if securityFailure = internalsecurity.AuthorizePrincipal(ctx, principal, def.Feature, def.Permission, opts.Authorizer); securityFailure != nil {
				applyProviderChallenge(w, provider, securityFailure)
				writeFailure(core.Failure{Status: securityFailure.Status, Code: securityFailure.Code, Detail: securityFailure.Detail})
				return
			}
			r = r.WithContext(ctx)
		} else if def.Feature != "" {
			next, securityFailure := internalsecurity.AuthenticateAndAuthorize(ctx, r.Header.Get("Authorization"), def.Feature, def.Permission, opts.Authenticator, opts.Authorizer)
			if securityFailure != nil {
				if securityFailure.Challenge != "" {
					w.Header().Set("WWW-Authenticate", securityFailure.Challenge)
				}
				writeFailure(core.Failure{Status: securityFailure.Status, Code: securityFailure.Code, Detail: securityFailure.Detail})
				return
			}
			ctx = next
			r = r.WithContext(ctx)
		}

		endpoint.ServeHTTP(w, r)
	})

	return Compiled{
		Pattern:      pattern,
		Handler:      handler,
		Operation:    oasOperation,
		Protected:    effectiveSecurityName != "",
		SecurityName: effectiveSecurityName,
	}, nil
}

func typedEndpoint(def *Definition, bindPlan *binding.Plan, validationPlan *validation.Plan, opts Options) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		report := func(err error) {
			if opts.ErrorHandler != nil {
				opts.ErrorHandler(ctx, err)
			}
		}
		writeFailure := func(item core.Failure) {
			internalfailure.WriteResolved(w, opts.FailureFormatter, opts.FailureContentType, item, report)
		}

		input, requestErr, fieldErrors := bindPlan.Bind(r)
		if requestErr != nil {
			writeFailure(core.Failure{Status: requestErr.Status, Code: requestErr.Code, Detail: requestErr.Detail})
			return
		}
		if len(fieldErrors) > 0 {
			writeFailure(validationFailure(fieldErrors))
			return
		}

		if def.Validation {
			fieldErrors = validationPlan.Validate(input.Interface())
			if opts.Validator != nil {
				fieldErrors = append(fieldErrors, opts.Validator.Validate(ctx, input.Interface())...)
			}
			if len(fieldErrors) > 0 {
				writeFailure(validationFailure(fieldErrors))
				return
			}
		}

		result := def.Invoke(ctx, input)
		if result == nil {
			writeFailure(core.Failure{Status: http.StatusInternalServerError, Code: "NIL_RESULT", Detail: "The operation returned no result"})
			return
		}
		result.WriteHTTPWithFailureFormatter(w, report, opts.FailureFormatter, opts.FailureContentType)
	})
}

func rawEndpoint(handler http.Handler, pattern route.Pattern, opts Options) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fieldErrors := make([]core.FieldError, 0)
		for _, parameter := range pattern.Parameters {
			raw := r.PathValue(parameter.Name)
			if raw == "" {
				fieldErrors = append(fieldErrors, core.FieldError{Location: "path", Field: parameter.Name, Messages: []string{"is required"}})
				continue
			}
			constraint := route.Constraints[parameter.Constraint]
			if constraint.Validate == nil {
				continue
			}
			if err := constraint.Validate(raw); err != nil {
				fieldErrors = append(fieldErrors, core.FieldError{
					Location: "path",
					Field:    parameter.Name,
					Messages: []string{fmt.Sprintf("does not satisfy %s constraint", parameter.Constraint)},
				})
			}
		}
		if len(fieldErrors) > 0 {
			ctx := r.Context()
			report := func(err error) {
				if opts.ErrorHandler != nil {
					opts.ErrorHandler(ctx, err)
				}
			}
			internalfailure.WriteResolved(w, opts.FailureFormatter, opts.FailureContentType, validationFailure(fieldErrors), report)
			return
		}
		handler.ServeHTTP(w, r)
	})
}

func compileOAS(def *Definition, plan *binding.Plan, pattern route.Pattern, registry *schema.Registry, failureContentType string, failureModelType reflect.Type) (*oas31.Operation, error) {
	if registry == nil {
		return nil, fmt.Errorf("schema registry is nil")
	}
	if def.OperationID == "" {
		return nil, fmt.Errorf("operation ID is required")
	}

	var params []binding.ParameterDoc
	var body *binding.BodyDoc
	if plan != nil {
		params, body = plan.Documentation()
	}

	success := false
	responses := map[string]oas31.Response{}
	for status, spec := range def.Responses {
		if status >= 200 && status < 300 {
			success = true
		}
		description := responseDescription(status, spec.Description)
		response := oas31.Response{Description: description}
		var err error
		if statusAllowsResponseBody(status) {
			switch spec.Kind {
			case ResponseProblem:
				response, err = documentedResponse(registry, description, failureContentType, failureModelType)
			case ResponseCustom:
				mediaType, _, _ := mime.ParseMediaType(strings.TrimSpace(spec.ContentType))
				response, err = documentedResponse(registry, description, mediaType, spec.ModelType)
			case ResponseRaw:
				// Raw handlers declare only status/description unless the caller uses ProducesResponse.
			default:
				response, err = documentedResponse(registry, description, "application/json", def.OutputType)
			}
			if err != nil {
				return nil, err
			}
		}
		responses[strconv.Itoa(status)] = response
	}
	if !success {
		return nil, fmt.Errorf("at least one successful response must be declared")
	}

	if plan != nil || rawPathHasValidation(pattern) {
		if err := addFailureResponse(responses, registry, http.StatusBadRequest, failureContentType, failureModelType); err != nil {
			return nil, err
		}
		if body != nil {
			if err := addFailureResponse(responses, registry, http.StatusRequestEntityTooLarge, failureContentType, failureModelType); err != nil {
				return nil, err
			}
			if err := addFailureResponse(responses, registry, http.StatusUnsupportedMediaType, failureContentType, failureModelType); err != nil {
				return nil, err
			}
		}
	}
	if def.SecurityName != "" || def.Feature != "" {
		if err := addFailureResponse(responses, registry, http.StatusUnauthorized, failureContentType, failureModelType); err != nil {
			return nil, err
		}
	}
	if def.Feature != "" {
		if err := addFailureResponse(responses, registry, http.StatusForbidden, failureContentType, failureModelType); err != nil {
			return nil, err
		}
	}
	if err := addFailureResponse(responses, registry, http.StatusInternalServerError, failureContentType, failureModelType); err != nil {
		return nil, err
	}

	operation := &oas31.Operation{
		OperationID: def.OperationID,
		Tags:        append([]string(nil), def.Tags...),
		Summary:     def.Summary,
		Description: def.Description,
		Responses:   responses,
	}

	routeConstraints := map[string]route.Constraint{}
	for _, p := range pattern.Parameters {
		routeConstraints[p.Name] = route.Constraints[p.Constraint]
	}

	if plan != nil {
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
	} else {
		for _, p := range pattern.Parameters {
			c := routeConstraints[p.Name]
			s := oas31.Schema{"type": c.JSONType}
			if c.Format != "" {
				s["format"] = c.Format
			}
			operation.Parameters = append(operation.Parameters, oas31.Parameter{Name: p.Name, In: "path", Required: true, Schema: &s})
		}
	}

	if body != nil {
		ref, err := registry.Ref(body.Type)
		if err != nil {
			return nil, fmt.Errorf("request body: %w", err)
		}
		operation.RequestBody = &oas31.RequestBody{Required: body.Required, Content: map[string]oas31.MediaType{"application/json": {Schema: ref}}}
	}
	if len(def.RawRequestMediaTypes) > 0 {
		content := make(map[string]oas31.MediaType, len(def.RawRequestMediaTypes))
		for _, raw := range def.RawRequestMediaTypes {
			mediaType, _, _ := mime.ParseMediaType(strings.TrimSpace(raw))
			content[mediaType] = oas31.MediaType{}
		}
		operation.RequestBody = &oas31.RequestBody{Required: true, Content: content}
	}

	securityName := def.SecurityName
	if securityName == "" && def.Feature != "" {
		securityName = "bearerAuth"
	}
	if securityName != "" {
		operation.Security = []map[string][]string{{securityName: {}}}
	}
	if def.Feature != "" {
		operation.Extensions = map[string]any{"x-feature": def.Feature, "x-permission": def.Permission}
	}
	return operation, nil
}

func rawPathHasValidation(pattern route.Pattern) bool {
	for _, parameter := range pattern.Parameters {
		if parameter.Constraint != "string" {
			return true
		}
	}
	return false
}

func responseDescription(status int, description string) string {
	if description != "" {
		return description
	}
	if text := http.StatusText(status); text != "" {
		return text
	}
	return "Response"
}

func documentedResponse(registry *schema.Registry, description, contentType string, modelType reflect.Type) (oas31.Response, error) {
	ref, err := registry.Ref(modelType)
	if err != nil {
		return oas31.Response{}, err
	}
	return oas31.Response{Description: description, Content: map[string]oas31.MediaType{contentType: {Schema: ref}}}, nil
}

func statusAllowsResponseBody(status int) bool {
	return status >= 200 && status != http.StatusNoContent && status != http.StatusNotModified
}

func addFailureResponse(responses map[string]oas31.Response, registry *schema.Registry, status int, contentType string, modelType reflect.Type) error {
	key := strconv.Itoa(status)
	if _, exists := responses[key]; exists {
		return nil
	}
	response, err := documentedResponse(registry, responseDescription(status, ""), contentType, modelType)
	if err != nil {
		return err
	}
	responses[key] = response
	return nil
}

func validationFailure(errors []core.FieldError) core.Failure {
	grouped := map[string][]string{}
	for _, field := range errors {
		key := strings.Trim(strings.TrimSpace(field.Location)+"."+strings.TrimSpace(field.Field), ".")
		if key == "" {
			key = "request"
		}
		grouped[key] = append(grouped[key], field.Messages...)
	}
	return core.Failure{
		Title:  "Bad Request",
		Status: http.StatusBadRequest,
		Code:   "VALIDATION_FAILED",
		Detail: "The request is invalid",
		Errors: grouped,
	}
}
