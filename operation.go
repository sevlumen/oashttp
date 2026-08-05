package oashttp

import (
	"net/http"
	"reflect"

	internaloperation "github.com/quang020102/go-osm/internal/operation"
)

type OperationBuilder[T any] struct {
	app        *App
	definition *internaloperation.Definition
}

func (b *OperationBuilder[T]) mutate(fn func(*internaloperation.Definition)) *OperationBuilder[T] {
	b.app.mutateOperation(b.definition, fn)
	return b
}
func (b *OperationBuilder[T]) WithOperationID(value string) *OperationBuilder[T] {
	return b.mutate(func(d *internaloperation.Definition) { d.OperationID = value })
}
func (b *OperationBuilder[T]) WithValidation() *OperationBuilder[T] {
	return b.mutate(func(d *internaloperation.Definition) { d.Validation = true })
}
func (b *OperationBuilder[T]) WithTags(values ...string) *OperationBuilder[T] {
	return b.mutate(func(d *internaloperation.Definition) { d.Tags = append([]string(nil), values...) })
}
func (b *OperationBuilder[T]) WithSummary(value string) *OperationBuilder[T] {
	return b.mutate(func(d *internaloperation.Definition) { d.Summary = value })
}
func (b *OperationBuilder[T]) WithDescription(value string) *OperationBuilder[T] {
	return b.mutate(func(d *internaloperation.Definition) { d.Description = value })
}
func (b *OperationBuilder[T]) Produces(status int) *OperationBuilder[T] {
	return b.ProducesJSON(status, http.StatusText(status))
}
func (b *OperationBuilder[T]) ProducesJSON(status int, description string) *OperationBuilder[T] {
	return b.mutate(func(d *internaloperation.Definition) {
		d.Responses[status] = internaloperation.ResponseSpec{Kind: internaloperation.ResponseJSON, Description: description}
	})
}

// ProducesProblem documents a failure using Config.FailureFormatter.
func (b *OperationBuilder[T]) ProducesProblem(status int) *OperationBuilder[T] {
	return b.mutate(func(d *internaloperation.Definition) {
		d.Responses[status] = internaloperation.ResponseSpec{Kind: internaloperation.ResponseProblem, Description: http.StatusText(status)}
	})
}

// ProducesResponse documents a caller-defined response body and media type.
// Pass a representative zero value such as APIError{} as model.
func (b *OperationBuilder[T]) ProducesResponse(status int, description, contentType string, model any) *OperationBuilder[T] {
	return b.mutate(func(d *internaloperation.Definition) {
		d.Responses[status] = internaloperation.ResponseSpec{
			Kind:        internaloperation.ResponseCustom,
			Description: description,
			ContentType: contentType,
			ModelType:   reflect.TypeOf(model),
		}
	})
}
func (b *OperationBuilder[T]) RequireFeatureAndPermission(feature, permission string) *OperationBuilder[T] {
	return b.mutate(func(d *internaloperation.Definition) { d.Feature = feature; d.Permission = permission })
}
