package operation

import (
	"context"
	"fmt"
	"net/http"
	"reflect"

	"github.com/sevlumen/oashttp/v2/internal/binding"
	"github.com/sevlumen/oashttp/v2/internal/core"
	internalfailure "github.com/sevlumen/oashttp/v2/internal/failure"
	"github.com/sevlumen/oashttp/v2/internal/oas31"
	"github.com/sevlumen/oashttp/v2/internal/route"
	"github.com/sevlumen/oashttp/v2/internal/schema"
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
	if err := validateDefinition(def, opts); err != nil {
		return Compiled{}, err
	}

	pattern, err := route.Parse(def.FullRoute)
	if err != nil {
		return Compiled{}, fmt.Errorf("%s %s: %w", def.Method, def.UserRoute, err)
	}

	bindPlan, validationPlan, err := compileRequestPlans(def, pattern, opts)
	if err != nil {
		return Compiled{}, err
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

	handler := compileRuntimeHandler(def, bindPlan, validationPlan, pattern, opts)

	effectiveSecurityName := def.SecurityName
	if effectiveSecurityName == "" && def.Feature != "" {
		effectiveSecurityName = "bearerAuth"
	}

	return Compiled{
		Pattern:      pattern,
		Handler:      handler,
		Operation:    oasOperation,
		Protected:    effectiveSecurityName != "",
		SecurityName: effectiveSecurityName,
	}, nil
}
