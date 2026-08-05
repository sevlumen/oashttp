package oashttp

import (
	"fmt"
	"strings"
)

type SwaggerUIConfig struct {
	Title       string
	DocumentURL string
	CDNVersion  string
	CSSURL      string
	BundleURL   string
}

func (a *App) MapOpenAPI(path string) error {
	if err := validateDocumentationPath(path); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.frozen {
		return ErrFrozen
	}
	if a.openAPIPath != "" {
		return fmt.Errorf("OpenAPI endpoint is already registered at %s", a.openAPIPath)
	}
	a.openAPIPath = path
	return nil
}
func (a *App) MapSwaggerUI(path string, config SwaggerUIConfig) error {
	if err := validateDocumentationPath(path); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.frozen {
		return ErrFrozen
	}
	for _, existing := range a.swagger {
		if existing.path == path {
			return fmt.Errorf("Swagger UI endpoint is already registered at %s", path)
		}
	}
	a.swagger = append(a.swagger, swaggerRegistration{path: path, config: config})
	return nil
}
func validateDocumentationPath(path string) error {
	if path == "" || !strings.HasPrefix(path, "/") {
		return fmt.Errorf("documentation path %q must start with /", path)
	}
	if strings.ContainsAny(path, "{}?#\x00") || strings.Contains(path, "//") {
		return fmt.Errorf("documentation path %q must be static", path)
	}
	return nil
}
