package operation

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/sevlumen/oashttp/v2/internal/core"
	"github.com/sevlumen/oashttp/v2/internal/schema"
)

type httpConformanceResult struct{}

func (httpConformanceResult) WriteHTTPWithFailureFormatter(http.ResponseWriter, func(error), core.FailureFormatter, string) {
}

func TestCompileRejectsTypedInformationalResponse(t *testing.T) {
	def := &Definition{
		Method:      http.MethodGet,
		UserRoute:   "/early",
		FullRoute:   "/early",
		InputType:   reflect.TypeOf(struct{}{}),
		OutputType:  reflect.TypeOf(struct{}{}),
		OperationID: "typedEarly",
		Responses: map[int]ResponseSpec{
			http.StatusEarlyHints: {Kind: ResponseJSON, Description: "Early Hints"},
			http.StatusNoContent:  {Kind: ResponseJSON, Description: "No Content"},
		},
		Invoke: func(context.Context, reflect.Value) core.ResultWriter { return httpConformanceResult{} },
	}

	_, err := Compile(def, Options{Registry: schema.NewRegistry()})
	if err == nil || !strings.Contains(err.Error(), "typed operation cannot declare informational response") {
		t.Fatalf("err=%v", err)
	}
}

func TestCompileAllowsRawInformationalResponse(t *testing.T) {
	def := &Definition{
		Method:      http.MethodGet,
		UserRoute:   "/raw-early",
		FullRoute:   "/raw-early",
		RawHandler:  http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
		OperationID: "rawEarly",
		Responses: map[int]ResponseSpec{
			http.StatusEarlyHints: {Kind: ResponseRaw, Description: "Early Hints"},
			http.StatusNoContent:  {Kind: ResponseRaw, Description: "No Content"},
		},
	}

	if _, err := Compile(def, Options{Registry: schema.NewRegistry()}); err != nil {
		t.Fatalf("Compile raw informational response: %v", err)
	}
}
