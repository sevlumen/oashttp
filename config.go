package oashttp

import (
	"context"
	"log"

	"github.com/sevlumen/oashttp/v2/internal/core"
)

type Server = core.Server
type FieldError = core.FieldError
type Validator = core.Validator

type SchemaProvider interface {
	JSONSchema() map[string]any
}

type Info struct {
	Title       string
	Version     string
	Description string
}

type Config struct {
	Info                   Info
	OpenAPIVersion         string
	JSONBodyLimit          int64
	AllowUnknownJSONFields bool
	Servers                []Server
	Validator              Validator
	ErrorHandler           func(context.Context, error)
	Authenticator          Authenticator
	Authorizer             Authorizer
	FailureFormatter       FailureFormatter

	// DisablePanicRecovery disables the default outer recovery middleware.
	// Production services should normally leave recovery enabled and provide an
	// ErrorHandler that reports recovered panics to their observability system.
	DisablePanicRecovery bool
}

type runtimeConfig struct {
	Config
	DisallowUnknownJSONFields bool
}

func normalizeConfig(cfg Config) runtimeConfig {
	if cfg.OpenAPIVersion == "" {
		cfg.OpenAPIVersion = "3.1.0"
	}
	if cfg.JSONBodyLimit == 0 {
		cfg.JSONBodyLimit = 1 << 20
	}
	if cfg.ErrorHandler == nil {
		cfg.ErrorHandler = func(_ context.Context, err error) { log.Printf("oashttp: %v", err) }
	}
	if cfg.FailureFormatter == nil {
		cfg.FailureFormatter = ProblemDetailsFormatter{}
	}
	return runtimeConfig{Config: cfg, DisallowUnknownJSONFields: !cfg.AllowUnknownJSONFields}
}
