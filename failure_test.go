package oashttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type apiErrorEnvelope struct {
	Success   bool         `json:"success"`
	Error     apiErrorBody `json:"error"`
	RequestID string       `json:"requestId,omitempty"`
}

type apiErrorBody struct {
	Code    string              `json:"code"`
	Message string              `json:"message"`
	Fields  map[string][]string `json:"fields,omitempty"`
}

type apiErrorFormatter struct{}

func (apiErrorFormatter) ContentType() string { return "application/json" }
func (apiErrorFormatter) Model() any          { return apiErrorEnvelope{} }
func (apiErrorFormatter) Format(f Failure) any {
	return apiErrorEnvelope{
		Success: false,
		Error: apiErrorBody{
			Code:    f.Code,
			Message: f.Detail,
			Fields:  f.Errors,
		},
		RequestID: f.TraceID,
	}
}

type failureInput struct {
	ID UUID `path:"id"`
}

type failureOutput struct {
	ID UUID `json:"id"`
}

func TestCustomFailureFormatterControlsRuntimeAndOpenAPI(t *testing.T) {
	app := New(Config{
		Info:             Info{Title: "Failures", Version: "1.0.1"},
		FailureFormatter: apiErrorFormatter{},
	})
	MapGet(app.Group(""), "/items/{id:uuid}", func(context.Context, failureInput) Result[failureOutput] {
		return NotFound[failureOutput]("ITEM_NOT_FOUND", "Item was not found")
	}).WithOperationID("getItem").Produces(http.StatusOK).ProducesProblem(http.StatusNotFound)
	if err := app.MapOpenAPI("/openapi.json"); err != nil {
		t.Fatal(err)
	}

	handler := app.MustBuild()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/items/not-a-uuid", nil)
	request.SetPathValue("id", "not-a-uuid")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type=%q", got)
	}
	var bindingFailure apiErrorEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &bindingFailure); err != nil {
		t.Fatal(err)
	}
	if bindingFailure.Success || bindingFailure.Error.Code != "VALIDATION_FAILED" {
		t.Fatalf("failure=%#v", bindingFailure)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/items/550e8400-e29b-41d4-a716-446655440000", nil)
	request.SetPathValue("id", "550e8400-e29b-41d4-a716-446655440000")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var businessFailure apiErrorEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &businessFailure); err != nil {
		t.Fatal(err)
	}
	if businessFailure.Error.Code != "ITEM_NOT_FOUND" {
		t.Fatalf("failure=%#v", businessFailure)
	}

	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	var document map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if _, exists := document["servers"]; exists {
		t.Fatal("servers must be omitted when Config.Servers is empty")
	}
	components := document["components"].(map[string]any)
	schemas := components["schemas"].(map[string]any)
	if _, exists := schemas["ProblemDetails"]; exists {
		t.Fatal("ProblemDetails must not be registered for a custom formatter")
	}
	if _, exists := schemas["apiErrorEnvelope"]; !exists {
		t.Fatalf("schemas=%#v", schemas)
	}
	paths := document["paths"].(map[string]any)
	operation := paths["/items/{id}"].(map[string]any)["get"].(map[string]any)
	responses := operation["responses"].(map[string]any)
	badRequest := responses["400"].(map[string]any)
	content := badRequest["content"].(map[string]any)
	if _, exists := content["application/json"]; !exists {
		t.Fatalf("content=%#v", content)
	}
}

func TestErrorJSONAndProducesResponseUseCallerDefinedBody(t *testing.T) {
	app := New(Config{Info: Info{Title: "Custom response", Version: "1.0.1"}})
	MapGet(app.Group(""), "/custom", func(context.Context, struct{}) Result[failureOutput] {
		return ErrorJSON[failureOutput](http.StatusConflict, apiErrorEnvelope{
			Success: false,
			Error:   apiErrorBody{Code: "NAME_CONFLICT", Message: "Name already exists"},
		})
	}).WithOperationID("customError").
		Produces(http.StatusOK).
		ProducesResponse(http.StatusConflict, "Name already exists", "application/json", apiErrorEnvelope{})
	if err := app.MapOpenAPI("/openapi.json"); err != nil {
		t.Fatal(err)
	}
	handler := app.MustBuild()

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/custom", nil))
	if recorder.Code != http.StatusConflict || recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status=%d headers=%v body=%s", recorder.Code, recorder.Header(), recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	var document map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	paths := document["paths"].(map[string]any)
	operation := paths["/custom"].(map[string]any)["get"].(map[string]any)
	responses := operation["responses"].(map[string]any)
	conflict := responses["409"].(map[string]any)
	schema := conflict["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
	if schema["$ref"] != "#/components/schemas/apiErrorEnvelope" {
		t.Fatalf("schema=%#v", schema)
	}
}
