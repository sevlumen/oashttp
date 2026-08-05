package oashttp

import (
	"net/http"

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
func (b *OperationBuilder[T]) ProducesProblem(status int) *OperationBuilder[T] {
	return b.mutate(func(d *internaloperation.Definition) {
		d.Responses[status] = internaloperation.ResponseSpec{Kind: internaloperation.ResponseProblem, Description: http.StatusText(status)}
	})
}
func (b *OperationBuilder[T]) RequireFeatureAndPermission(feature, permission string) *OperationBuilder[T] {
	return b.mutate(func(d *internaloperation.Definition) { d.Feature = feature; d.Permission = permission })
}
