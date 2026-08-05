package failure

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/quang020102/go-osm/internal/core"
)

type testBody struct {
	Code string `json:"code"`
}

type testFormatter struct{}

func (testFormatter) ContentType() string       { return "application/json; charset=utf-8" }
func (testFormatter) Model() any                { return testBody{} }
func (testFormatter) Format(f core.Failure) any { return testBody{Code: f.Code} }

type panicFormatter struct{}

func (panicFormatter) ContentType() string     { return "application/json" }
func (panicFormatter) Model() any              { return testBody{} }
func (panicFormatter) Format(core.Failure) any { panic("boom") }

type invalidFormatter struct{}

func (invalidFormatter) ContentType() string     { return "not a media type" }
func (invalidFormatter) Model() any              { return nil }
func (invalidFormatter) Format(core.Failure) any { return nil }

func TestDescribeAndWriteCustomFormatter(t *testing.T) {
	contentType, modelType, err := Describe(testFormatter{})
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "application/json" || modelType.Name() != "testBody" {
		t.Fatalf("contentType=%q model=%v", contentType, modelType)
	}
	recorder := httptest.NewRecorder()
	Write(recorder, testFormatter{}, core.Failure{Status: http.StatusBadRequest, Code: "INVALID"}, nil)
	if recorder.Code != http.StatusBadRequest || recorder.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("status=%d headers=%v", recorder.Code, recorder.Header())
	}
	var body testBody
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != "INVALID" {
		t.Fatalf("body=%#v", body)
	}
}

func TestFormatterFailureFallsBackToProblemDetails(t *testing.T) {
	var reported error
	recorder := httptest.NewRecorder()
	Write(recorder, panicFormatter{}, core.Failure{Status: http.StatusBadRequest, Code: "INVALID"}, func(err error) { reported = err })
	if recorder.Code != http.StatusInternalServerError || recorder.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("status=%d headers=%v body=%s", recorder.Code, recorder.Header(), recorder.Body.String())
	}
	if reported == nil || !strings.Contains(reported.Error(), "panicked") {
		t.Fatalf("reported=%v", reported)
	}
}

func TestDescribeRejectsInvalidFormatter(t *testing.T) {
	if _, _, err := Describe(invalidFormatter{}); err == nil {
		t.Fatal("expected formatter validation error")
	}
}
