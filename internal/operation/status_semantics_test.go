package operation

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"github.com/sevlumen/oashttp/v2/internal/core"
	"github.com/sevlumen/oashttp/v2/internal/schema"
)

func TestCompileResetContentDoesNotRequireOrDocumentResponseBody(t *testing.T) {
	def := &Definition{
		Method:      http.MethodGet,
		UserRoute:   "/reset",
		FullRoute:   "/reset",
		InputType:   reflect.TypeOf(struct{}{}),
		OutputType:  reflect.TypeOf(struct{}{}),
		OperationID: "reset",
		Responses: map[int]ResponseSpec{
			http.StatusResetContent: {
				Kind:        ResponseCustom,
				Description: "Reset Content",
			},
		},
		Invoke: func(context.Context, reflect.Value) core.ResultWriter { return compileResult{} },
	}

	compiled, err := Compile(def, Options{Registry: schema.NewRegistry()})
	if err != nil {
		t.Fatalf("Compile returned error for bodyless 205 response: %v", err)
	}
	response := compiled.Operation.Responses["205"]
	if len(response.Content) != 0 {
		t.Fatalf("205 response content=%#v", response.Content)
	}
}
