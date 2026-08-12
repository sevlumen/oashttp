package oashttp

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/sevlumen/oashttp/v2/internal/oas31"
	internaloperation "github.com/sevlumen/oashttp/v2/internal/operation"
)

func newOpenAPIDocument(cfg runtimeConfig) *oas31.Document {
	document := &oas31.Document{
		OpenAPI:    "3.1.0",
		Info:       oas31.Info{Title: cfg.Info.Title, Version: cfg.Info.Version, Description: cfg.Info.Description},
		Paths:      map[string]*oas31.PathItem{},
		Components: oas31.Components{Schemas: map[string]*oas31.Schema{}, Responses: map[string]oas31.Response{}, SecuritySchemes: map[string]oas31.SecurityScheme{}},
	}
	for _, server := range cfg.Servers {
		document.Servers = append(document.Servers, oas31.Server{URL: server.URL, Description: server.Description})
	}
	return document
}

func applyCompiledOperation(document *oas31.Document, def *internaloperation.Definition, compiled internaloperation.Compiled, cfg runtimeConfig) error {
	item := document.Paths[compiled.Pattern.OpenAPIPath]
	if item == nil {
		item = &oas31.PathItem{}
		document.Paths[compiled.Pattern.OpenAPIPath] = item
	}
	if err := setPathOperation(item, def.Method, compiled.Operation); err != nil {
		return err
	}

	if compiled.SecurityName == "" {
		return nil
	}
	if def.SecurityName == "" {
		document.Components.SecuritySchemes["bearerAuth"] = oas31.SecurityScheme{Type: "http", Scheme: "bearer", BearerFormat: "JWT"}
		return nil
	}

	provider := cfg.SecurityProviders[compiled.SecurityName]
	scheme, err := openAPISecurityScheme(compiled.SecurityName, provider)
	if err != nil {
		return err
	}
	document.Components.SecuritySchemes[compiled.SecurityName] = scheme
	return nil
}

func openAPISecurityScheme(name string, provider SecurityProvider) (oas31.SecurityScheme, error) {
	if provider == nil {
		return oas31.SecurityScheme{}, fmt.Errorf("security provider %q is not configured", name)
	}
	scheme := provider.SecurityScheme()
	result := oas31.SecurityScheme{
		Type:         strings.TrimSpace(scheme.Type),
		Scheme:       strings.TrimSpace(scheme.Scheme),
		BearerFormat: strings.TrimSpace(scheme.BearerFormat),
		Name:         strings.TrimSpace(scheme.Name),
		In:           strings.TrimSpace(scheme.In),
	}
	switch result.Type {
	case "http":
		if result.Scheme == "" {
			return oas31.SecurityScheme{}, fmt.Errorf("security provider %q: http scheme is required", name)
		}
		if result.Name != "" || result.In != "" {
			return oas31.SecurityScheme{}, fmt.Errorf("security provider %q: http schemes cannot declare apiKey name or in fields", name)
		}
		if result.BearerFormat != "" && !strings.EqualFold(result.Scheme, "bearer") {
			return oas31.SecurityScheme{}, fmt.Errorf("security provider %q: bearerFormat is valid only for http bearer schemes", name)
		}
	case "apiKey":
		if result.Name == "" {
			return oas31.SecurityScheme{}, fmt.Errorf("security provider %q: apiKey name is required", name)
		}
		if result.In != "header" && result.In != "query" && result.In != "cookie" {
			return oas31.SecurityScheme{}, fmt.Errorf("security provider %q: apiKey in must be header, query, or cookie", name)
		}
		if result.Scheme != "" || result.BearerFormat != "" {
			return oas31.SecurityScheme{}, fmt.Errorf("security provider %q: apiKey schemes cannot declare http scheme or bearerFormat fields", name)
		}
	default:
		return oas31.SecurityScheme{}, fmt.Errorf("security provider %q: unsupported OpenAPI security type %q", name, result.Type)
	}
	return result, nil
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
	case http.MethodOptions:
		item.Options = operation
	case http.MethodHead:
		item.Head = operation
	case http.MethodTrace:
		item.Trace = operation
	default:
		return fmt.Errorf("unsupported HTTP method %q", method)
	}
	return nil
}
