package operation

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/sevlumen/oashttp/v2/internal/binding"
	"github.com/sevlumen/oashttp/v2/internal/core"
	internalfailure "github.com/sevlumen/oashttp/v2/internal/failure"
	"github.com/sevlumen/oashttp/v2/internal/route"
	internalsecurity "github.com/sevlumen/oashttp/v2/internal/security"
	"github.com/sevlumen/oashttp/v2/internal/validation"
)

func compileRuntimeHandler(def *Definition, bindPlan *binding.Plan, validationPlan *validation.Plan, pattern route.Pattern, opts Options) http.Handler {
	var endpoint http.Handler
	if def.RawHandler != nil {
		endpoint = rawEndpoint(def.RawHandler, pattern, opts)
	} else {
		endpoint = typedEndpoint(def, bindPlan, validationPlan, opts)
	}
	for index := len(def.Middlewares) - 1; index >= 0; index-- {
		endpoint = def.Middlewares[index](endpoint)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
