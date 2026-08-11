package operation

import (
	"context"
	"net/http"
	"reflect"

	"github.com/sevlumen/oashttp/v2/internal/core"
)

type ResponseKind uint8

const (
	ResponseJSON ResponseKind = iota
	ResponseProblem
	ResponseCustom
)

type ResponseSpec struct {
	Kind        ResponseKind
	Description string
	ContentType string
	ModelType   reflect.Type
}

type Definition struct {
	Method      string
	UserRoute   string
	FullRoute   string
	InputType   reflect.Type
	OutputType  reflect.Type
	Invoke      func(context.Context, reflect.Value) core.ResultWriter
	RawHandler  http.Handler
	OperationID string
	Tags        []string
	Summary     string
	Description string
	Responses   map[int]ResponseSpec
	Validation  bool
	Feature     string
	Permission  string
	SecurityName string
	Middlewares []func(http.Handler) http.Handler
}
