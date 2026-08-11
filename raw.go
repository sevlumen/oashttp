package oashttp

import (
	"net/http"
	"reflect"

	internaloperation "github.com/sevlumen/oashttp/v2/internal/operation"
)

// RawOperationBuilder configures a standard net/http handler that participates
// in oashttp routing, middleware, security, operation metadata, and OpenAPI.
type RawOperationBuilder struct {
	app        *App
	definition *internaloperation.Definition
}

// MapHandler registers a standard net/http handler without typed request
// binding. It is intended for streaming and custom-representation endpoints.
func MapHandler(group *Group, method, path string, handler http.Handler) *RawOperationBuilder {
	if group == nil || group.app == nil {
		panic("oashttp: group is nil")
	}
	if handler == nil {
		panic("oashttp: handler is nil")
	}
	full := joinPaths(group.prefix, path)
	def := &internaloperation.Definition{
		Method:     method,
		UserRoute:  full,
		FullRoute:  full,
		RawHandler: handler,
		Responses:  map[int]internaloperation.ResponseSpec{},
	}
	for _, middleware := range group.middlewares {
		def.Middlewares = append(def.Middlewares, middleware)
	}
	group.app.registerOperation(def)
	return &RawOperationBuilder{app: group.app, definition: def}
}

func (b *RawOperationBuilder) mutate(fn func(*internaloperation.Definition)) *RawOperationBuilder {
	b.app.mutateOperation(b.definition, fn)
	return b
}
func (b *RawOperationBuilder) WithOperationID(value string) *RawOperationBuilder {
	return b.mutate(func(d *internaloperation.Definition) { d.OperationID = value })
}
func (b *RawOperationBuilder) WithTags(values ...string) *RawOperationBuilder {
	return b.mutate(func(d *internaloperation.Definition) { d.Tags = append([]string(nil), values...) })
}
func (b *RawOperationBuilder) WithSummary(value string) *RawOperationBuilder {
	return b.mutate(func(d *internaloperation.Definition) { d.Summary = value })
}
func (b *RawOperationBuilder) WithDescription(value string) *RawOperationBuilder {
	return b.mutate(func(d *internaloperation.Definition) { d.Description = value })
}
func (b *RawOperationBuilder) Use(values ...Middleware) *RawOperationBuilder {
	return b.mutate(func(d *internaloperation.Definition) {
		for _, middleware := range values {
			if middleware == nil {
				panic("oashttp: middleware is nil")
			}
			d.Middlewares = append(d.Middlewares, middleware)
		}
	})
}

// Consumes documents request media types for a raw handler. The raw handler
// remains responsible for Content-Type enforcement and body parsing/streaming.
func (b *RawOperationBuilder) Consumes(contentTypes ...string) *RawOperationBuilder {
	return b.mutate(func(d *internaloperation.Definition) {
		d.RawRequestMediaTypes = append([]string(nil), contentTypes...)
	})
}

func (b *RawOperationBuilder) Produces(status int) *RawOperationBuilder {
	return b.mutate(func(d *internaloperation.Definition) {
		d.Responses[status] = internaloperation.ResponseSpec{Kind: internaloperation.ResponseRaw, Description: http.StatusText(status)}
	})
}
func (b *RawOperationBuilder) ProducesProblem(status int) *RawOperationBuilder {
	return b.mutate(func(d *internaloperation.Definition) {
		d.Responses[status] = internaloperation.ResponseSpec{Kind: internaloperation.ResponseProblem, Description: http.StatusText(status)}
	})
}
func (b *RawOperationBuilder) ProducesResponse(status int, description, contentType string, model any) *RawOperationBuilder {
	return b.mutate(func(d *internaloperation.Definition) {
		d.Responses[status] = internaloperation.ResponseSpec{
			Kind:        internaloperation.ResponseCustom,
			Description: description,
			ContentType: contentType,
			ModelType:   reflect.TypeOf(model),
		}
	})
}
func (b *RawOperationBuilder) RequireSecurity(name string) *RawOperationBuilder {
	name = normalizeSecurityName(name)
	return b.mutate(func(d *internaloperation.Definition) { d.SecurityName = name })
}
func (b *RawOperationBuilder) RequireFeatureAndPermission(feature, permission string) *RawOperationBuilder {
	return b.mutate(func(d *internaloperation.Definition) { d.Feature = feature; d.Permission = permission })
}
