package operation

import (
	"context"
	"reflect"

	"github.com/oashttp/oashttp/internal/core"
)

type ResponseKind uint8

const (
	ResponseJSON ResponseKind = iota
	ResponseProblem
)

type ResponseSpec struct {
	Kind        ResponseKind
	Description string
}
type Definition struct {
	Method      string
	UserRoute   string
	FullRoute   string
	InputType   reflect.Type
	OutputType  reflect.Type
	Invoke      func(context.Context, reflect.Value) core.ResultWriter
	OperationID string
	Tags        []string
	Summary     string
	Description string
	Responses   map[int]ResponseSpec
	Validation  bool
	Feature     string
	Permission  string
}
