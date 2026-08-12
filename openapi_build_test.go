package oashttp

import (
	"context"
	"net/http"
	"testing"

	"github.com/sevlumen/oashttp/v2/internal/oas31"
)

type openAPIBuildTestProvider struct {
	scheme SecurityScheme
}

func (p openAPIBuildTestProvider) SecurityScheme() SecurityScheme { return p.scheme }
func (openAPIBuildTestProvider) Authenticate(context.Context, *http.Request) (*Principal, error) {
	return nil, nil
}

func TestSetPathOperationSupportsAllOpenAPIHTTPMethods(t *testing.T) {
	operation := &oas31.Operation{OperationID: "test"}
	tests := []struct {
		method string
		get    func(*oas31.PathItem) *oas31.Operation
	}{
		{http.MethodGet, func(item *oas31.PathItem) *oas31.Operation { return item.Get }},
		{http.MethodPost, func(item *oas31.PathItem) *oas31.Operation { return item.Post }},
		{http.MethodPut, func(item *oas31.PathItem) *oas31.Operation { return item.Put }},
		{http.MethodPatch, func(item *oas31.PathItem) *oas31.Operation { return item.Patch }},
		{http.MethodDelete, func(item *oas31.PathItem) *oas31.Operation { return item.Delete }},
		{http.MethodOptions, func(item *oas31.PathItem) *oas31.Operation { return item.Options }},
		{http.MethodHead, func(item *oas31.PathItem) *oas31.Operation { return item.Head }},
		{http.MethodTrace, func(item *oas31.PathItem) *oas31.Operation { return item.Trace }},
	}

	for _, test := range tests {
		t.Run(test.method, func(t *testing.T) {
			item := &oas31.PathItem{}
			if err := setPathOperation(item, test.method, operation); err != nil {
				t.Fatal(err)
			}
			if test.get(item) != operation {
				t.Fatalf("operation not assigned for %s", test.method)
			}
		})
	}
}

func TestOpenAPISecuritySchemeRejectsFieldsFromAnotherSchemeType(t *testing.T) {
	tests := []struct {
		name   string
		scheme SecurityScheme
	}{
		{
			name:   "http with apiKey fields",
			scheme: SecurityScheme{Type: "http", Scheme: "bearer", Name: "X-Key", In: "header"},
		},
		{
			name:   "apiKey with http fields",
			scheme: SecurityScheme{Type: "apiKey", Name: "X-Key", In: "header", Scheme: "bearer"},
		},
		{
			name:   "non-bearer with bearer format",
			scheme: SecurityScheme{Type: "http", Scheme: "basic", BearerFormat: "JWT"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := openAPISecurityScheme("test", openAPIBuildTestProvider{scheme: test.scheme}); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
