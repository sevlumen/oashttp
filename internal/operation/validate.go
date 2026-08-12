package operation

import (
	"fmt"
	"mime"
	"net/http"
	"strings"

	"github.com/sevlumen/oashttp/v2/internal/binding"
	"github.com/sevlumen/oashttp/v2/internal/route"
	"github.com/sevlumen/oashttp/v2/internal/validation"
)

func validateDefinition(def *Definition, opts Options) error {
	if strings.TrimSpace(def.OperationID) == "" {
		return fmt.Errorf("%s %s: operation ID is required", def.Method, def.UserRoute)
	}
	if (def.Feature == "") != (def.Permission == "") {
		return fmt.Errorf("%s %s: feature and permission must be configured together", def.Method, def.UserRoute)
	}
	if def.SecurityName != "" {
		provider, ok := opts.SecurityProviders[def.SecurityName]
		if !ok || provider == nil {
			return fmt.Errorf("%s %s: security provider %q is not configured", def.Method, def.UserRoute, def.SecurityName)
		}
	}
	if def.Feature != "" && def.SecurityName == "" && opts.Authenticator == nil {
		return fmt.Errorf("%s %s: protected operation requires Config.Authenticator or RequireSecurity", def.Method, def.UserRoute)
	}
	for status, spec := range def.Responses {
		if status < 100 || status > 599 {
			return fmt.Errorf("%s %s: invalid response status %d", def.Method, def.UserRoute, status)
		}
		if spec.Kind == ResponseCustom && statusAllowsResponseBody(status) {
			if spec.ModelType == nil {
				return fmt.Errorf("%s %s: custom response %d requires a non-nil model", def.Method, def.UserRoute, status)
			}
			mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(spec.ContentType))
			if err != nil || mediaType == "" {
				return fmt.Errorf("%s %s: custom response %d has invalid content type %q", def.Method, def.UserRoute, status, spec.ContentType)
			}
		}
	}
	for _, contentType := range def.RawRequestMediaTypes {
		mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(contentType))
		if err != nil || mediaType == "" {
			return fmt.Errorf("%s %s: raw request has invalid content type %q", def.Method, def.UserRoute, contentType)
		}
	}
	return nil
}

func compileRequestPlans(def *Definition, pattern route.Pattern, opts Options) (*binding.Plan, *validation.Plan, error) {
	if def.RawHandler != nil {
		return nil, nil, nil
	}
	if def.InputType == nil || def.OutputType == nil || def.Invoke == nil {
		return nil, nil, fmt.Errorf("%s %s: typed operation is incomplete", def.Method, def.UserRoute)
	}

	bindPlan, err := binding.Compile(def.InputType, pattern, opts.Binding)
	if err != nil {
		return nil, nil, fmt.Errorf("%s %s (%s -> %s): %w", def.Method, def.UserRoute, def.InputType, def.OutputType, err)
	}

	var validationPlan *validation.Plan
	if def.Validation {
		validationPlan, err = validation.Compile(def.InputType)
		if err != nil {
			return nil, nil, fmt.Errorf("%s %s (%s): %w", def.Method, def.UserRoute, def.InputType, err)
		}
	}
	return bindPlan, validationPlan, nil
}

func statusAllowsResponseBody(status int) bool {
	return status >= 200 && status != http.StatusNoContent && status != http.StatusNotModified
}
