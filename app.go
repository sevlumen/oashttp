package oashttp

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/quang020102/go-osm/internal/binding"
	internaldocs "github.com/quang020102/go-osm/internal/docs"
	internalfailure "github.com/quang020102/go-osm/internal/failure"
	"github.com/quang020102/go-osm/internal/oas31"
	internaloperation "github.com/quang020102/go-osm/internal/operation"
	"github.com/quang020102/go-osm/internal/schema"
)

var ErrFrozen = errors.New("oashttp application is frozen")

type swaggerRegistration struct {
	path   string
	config SwaggerUIConfig
}

type App struct {
	mu           sync.Mutex
	buildOnce    sync.Once
	config       runtimeConfig
	frozen       bool
	middlewares  []Middleware
	operations   []*internaloperation.Definition
	openAPIPath  string
	swagger      []swaggerRegistration
	builtHandler http.Handler
	builtErr     error
}

func New(config Config) *App { return &App{config: normalizeConfig(config)} }

func (a *App) Use(middleware Middleware) error {
	if middleware == nil {
		return fmt.Errorf("middleware cannot be nil")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.frozen {
		return ErrFrozen
	}
	a.middlewares = append(a.middlewares, middleware)
	return nil
}

func (a *App) registerOperation(def *internaloperation.Definition) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.frozen {
		panic(ErrFrozen)
	}
	a.operations = append(a.operations, def)
}

func (a *App) mutateOperation(def *internaloperation.Definition, mutate func(*internaloperation.Definition)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.frozen {
		panic(ErrFrozen)
	}
	mutate(def)
}

func (a *App) Build() (http.Handler, error) {
	a.buildOnce.Do(a.build)
	return a.builtHandler, a.builtErr
}

func (a *App) MustBuild() http.Handler {
	handler, err := a.Build()
	if err != nil {
		panic(err)
	}
	return handler
}

func (a *App) build() {
	a.mu.Lock()
	a.frozen = true
	cfg := a.config
	operations := append([]*internaloperation.Definition(nil), a.operations...)
	middlewares := append([]Middleware(nil), a.middlewares...)
	openAPIPath := a.openAPIPath
	swagger := append([]swaggerRegistration(nil), a.swagger...)
	a.mu.Unlock()

	if strings.TrimSpace(cfg.Info.Title) == "" {
		a.builtErr = fmt.Errorf("oashttp: Info.Title is required")
		return
	}
	if strings.TrimSpace(cfg.Info.Version) == "" {
		a.builtErr = fmt.Errorf("oashttp: Info.Version is required")
		return
	}
	if cfg.OpenAPIVersion != "3.1.0" {
		a.builtErr = fmt.Errorf("oashttp: OpenAPIVersion must be exactly 3.1.0")
		return
	}
	if cfg.JSONBodyLimit <= 0 {
		a.builtErr = fmt.Errorf("oashttp: JSONBodyLimit must be positive")
		return
	}

	document := &oas31.Document{
		OpenAPI:    "3.1.0",
		Info:       oas31.Info{Title: cfg.Info.Title, Version: cfg.Info.Version, Description: cfg.Info.Description},
		Paths:      map[string]*oas31.PathItem{},
		Components: oas31.Components{Schemas: map[string]*oas31.Schema{}, Responses: map[string]oas31.Response{}, SecuritySchemes: map[string]oas31.SecurityScheme{}},
	}
	for _, server := range cfg.Servers {
		document.Servers = append(document.Servers, oas31.Server{URL: server.URL, Description: server.Description})
	}

	failureContentType, failureModelType, err := internalfailure.Describe(cfg.FailureFormatter)
	if err != nil {
		a.builtErr = fmt.Errorf("oashttp: invalid FailureFormatter: %w", err)
		return
	}

	registry := schema.NewRegistry()
	mux := http.NewServeMux()
	registered := map[string]string{}
	operationIDs := map[string]string{}
	protected := false

	for _, def := range operations {
		if previous, ok := operationIDs[def.OperationID]; def.OperationID != "" && ok {
			a.builtErr = fmt.Errorf("duplicate operation ID %q on %s and %s", def.OperationID, previous, def.Method+" "+def.UserRoute)
			return
		}
		if def.OperationID != "" {
			operationIDs[def.OperationID] = def.Method + " " + def.UserRoute
		}
		compiled, err := internaloperation.Compile(def, internaloperation.Options{
			Binding:            binding.Options{JSONBodyLimit: cfg.JSONBodyLimit, DisallowUnknownJSONFields: cfg.DisallowUnknownJSONFields},
			Registry:           registry,
			Validator:          cfg.Validator,
			Authenticator:      cfg.Authenticator,
			Authorizer:         cfg.Authorizer,
			ErrorHandler:       cfg.ErrorHandler,
			FailureFormatter:   cfg.FailureFormatter,
			FailureContentType: failureContentType,
			FailureModelType:   failureModelType,
		})
		if err != nil {
			a.builtErr = err
			return
		}
		key := def.Method + " " + compiled.Pattern.ServeMuxPath
		if previous, ok := registered[key]; ok {
			a.builtErr = fmt.Errorf("duplicate route %s registered by %s and %s", key, previous, def.InputType)
			return
		}
		registered[key] = def.InputType.String()
		if err := handleMuxPattern(mux, key, compiled.Handler); err != nil {
			a.builtErr = fmt.Errorf("register %s: %w", key, err)
			return
		}
		item := document.Paths[compiled.Pattern.OpenAPIPath]
		if item == nil {
			item = &oas31.PathItem{}
			document.Paths[compiled.Pattern.OpenAPIPath] = item
		}
		if err := setPathOperation(item, def.Method, compiled.Operation); err != nil {
			a.builtErr = err
			return
		}
		protected = protected || compiled.Protected
	}
	document.Components.Schemas = registry.Components()
	if protected {
		document.Components.SecuritySchemes["bearerAuth"] = oas31.SecurityScheme{Type: "http", Scheme: "bearer", BearerFormat: "JWT"}
	}

	openAPIBytes, err := oas31.Marshal(document)
	if err != nil {
		a.builtErr = fmt.Errorf("marshal OpenAPI document: %w", err)
		return
	}
	if openAPIPath != "" {
		key := http.MethodGet + " " + openAPIPath
		if _, ok := registered[key]; ok {
			a.builtErr = fmt.Errorf("documentation route conflicts with %s", key)
			return
		}
		registered[key] = "OpenAPI"
		if err := handleMuxPattern(mux, key, internaldocs.NewOpenAPIHandler(openAPIBytes)); err != nil {
			a.builtErr = fmt.Errorf("register %s: %w", key, err)
			return
		}
	}
	for _, registration := range swagger {
		config := internaldocs.Config(registration.config)
		if config.DocumentURL == "" {
			if openAPIPath != "" {
				config.DocumentURL = openAPIPath
			} else {
				config.DocumentURL = "/openapi.json"
			}
		}
		handler, err := internaldocs.NewSwaggerHandler(config)
		if err != nil {
			a.builtErr = fmt.Errorf("swagger %s: %w", registration.path, err)
			return
		}
		key := http.MethodGet + " " + registration.path
		if _, ok := registered[key]; ok {
			a.builtErr = fmt.Errorf("documentation route conflicts with %s", key)
			return
		}
		registered[key] = "Swagger UI"
		if err := handleMuxPattern(mux, key, handler); err != nil {
			a.builtErr = fmt.Errorf("register %s: %w", key, err)
			return
		}
	}

	var handler http.Handler = mux
	for index := len(middlewares) - 1; index >= 0; index-- {
		handler = middlewares[index](handler)
	}
	if !cfg.DisablePanicRecovery {
		handler = recoverPanics(handler, cfg.ErrorHandler, cfg.FailureFormatter, failureContentType)
	}
	a.builtHandler = handler
}

func setPathOperation(item *oas31.PathItem, method string, operation *oas31.Operation) error {
	switch method {
	case http.MethodGet:
		item.Get = operation
	case http.MethodPost:
		item.Post = operation
	case http.MethodPut:
		item.Put = operation
	case http.MethodPatch:
		item.Patch = operation
	case http.MethodDelete:
		item.Delete = operation
	default:
		return fmt.Errorf("unsupported HTTP method %q", method)
	}
	return nil
}

func handleMuxPattern(mux *http.ServeMux, pattern string, handler http.Handler) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("invalid or conflicting ServeMux pattern: %v", recovered)
		}
	}()
	mux.Handle(pattern, handler)
	return nil
}
