package oashttp

import (
	"context"
	"net/http"
	"reflect"

	"github.com/quang020102/go-osm/internal/core"
	internaloperation "github.com/quang020102/go-osm/internal/operation"
)

func MapGet[I any, O any](group *Group, path string, handler func(context.Context, I) Result[O]) *OperationBuilder[O] {
	return mapOperation(group, http.MethodGet, path, handler)
}
func MapPost[I any, O any](group *Group, path string, handler func(context.Context, I) Result[O]) *OperationBuilder[O] {
	return mapOperation(group, http.MethodPost, path, handler)
}
func MapPut[I any, O any](group *Group, path string, handler func(context.Context, I) Result[O]) *OperationBuilder[O] {
	return mapOperation(group, http.MethodPut, path, handler)
}
func MapPatch[I any, O any](group *Group, path string, handler func(context.Context, I) Result[O]) *OperationBuilder[O] {
	return mapOperation(group, http.MethodPatch, path, handler)
}
func MapDelete[I any, O any](group *Group, path string, handler func(context.Context, I) Result[O]) *OperationBuilder[O] {
	return mapOperation(group, http.MethodDelete, path, handler)
}

func mapOperation[I any, O any](group *Group, method, path string, handler func(context.Context, I) Result[O]) *OperationBuilder[O] {
	if group == nil || group.app == nil {
		panic("oashttp: group is nil")
	}
	if handler == nil {
		panic("oashttp: handler is nil")
	}
	full := joinPaths(group.prefix, path)
	inputType := reflect.TypeOf((*I)(nil)).Elem()
	outputType := reflect.TypeOf((*O)(nil)).Elem()
	def := &internaloperation.Definition{Method: method, UserRoute: full, FullRoute: full, InputType: inputType, OutputType: outputType, Responses: map[int]internaloperation.ResponseSpec{}}
	def.Invoke = func(ctx context.Context, value reflect.Value) core.ResultWriter {
		input := value.Interface().(I)
		return handler(ctx, input)
	}
	group.app.registerOperation(def)
	return &OperationBuilder[O]{app: group.app, definition: def}
}
