package oashttp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRawHandlerDocumentsConsumedMediaTypes(t *testing.T) {
	app := New(Config{Info: Info{Title: "Raw body", Version: "1.0.0"}})
	MapHandler(app.Group(""), http.MethodPost, "/upload", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).WithOperationID("uploadRawBody").Consumes("application/octet-stream").Produces(http.StatusNoContent)
	if err := app.MapOpenAPI("/openapi.json"); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	app.MustBuild().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	body := recorder.Body.String()
	for _, expected := range []string{`"requestBody"`, `"application/octet-stream":{}`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("OpenAPI missing %s: %s", expected, body)
		}
	}
	if strings.Contains(body, `"required":true`) {
		t.Fatalf("raw Consumes must not require an application-owned request body: %s", body)
	}
}

func TestRawHandlerRejectsInvalidConsumedMediaTypeAtBuild(t *testing.T) {
	app := New(Config{Info: Info{Title: "Raw body", Version: "1.0.0"}})
	MapHandler(app.Group(""), http.MethodPost, "/upload", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).WithOperationID("uploadInvalidBody").Consumes("not a media type").Produces(http.StatusNoContent)
	if _, err := app.Build(); err == nil {
		t.Fatal("expected invalid consumed media type build error")
	}
}
