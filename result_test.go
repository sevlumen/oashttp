package oashttp

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResultWritesJSONSuccess(t *testing.T) {
	r := httptest.NewRecorder()
	OK(struct {
		ID string `json:"id"`
	}{"u-1"}).WithHeader("ETag", `"v1"`).write(r)
	if r.Code != http.StatusOK {
		t.Fatalf("status=%d", r.Code)
	}
	if got := r.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type=%q", got)
	}
	if got := r.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options=%q", got)
	}
	if got := r.Header().Get("ETag"); got != `"v1"` {
		t.Fatalf("ETag=%q", got)
	}
	if strings.TrimSpace(r.Body.String()) != `{"id":"u-1"}` {
		t.Fatalf("body=%q", r.Body.String())
	}
}

func TestProblemWritesProblemJSON(t *testing.T) {
	r := httptest.NewRecorder()
	NotFound[struct{}]("USER_NOT_FOUND", "User was not found").write(r)
	if r.Code != http.StatusNotFound {
		t.Fatalf("status=%d", r.Code)
	}
	if got := r.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Fatalf("Content-Type=%q", got)
	}
	if got := r.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control=%q", got)
	}
}

func TestNoContentDoesNotWritePayloadHeaders(t *testing.T) {
	r := httptest.NewRecorder()
	NoContent[struct{}]().write(r)
	if r.Code != http.StatusNoContent || r.Body.Len() != 0 {
		t.Fatalf("status=%d body=%q", r.Code, r.Body.String())
	}
	if contentType := r.Header().Get("Content-Type"); contentType != "" {
		t.Fatalf("Content-Type=%q", contentType)
	}
}

func TestResultSerializationFailureReturns500BeforeCommit(t *testing.T) {
	type invalid struct {
		Value chan int `json:"value"`
	}
	var reported error
	r := httptest.NewRecorder()
	OK(invalid{Value: make(chan int)}).WriteHTTP(r, func(err error) { reported = err })
	if r.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", r.Code, r.Body.String())
	}
	if reported == nil {
		t.Fatal("expected serialization error")
	}
	var problem ProblemDetails
	if err := json.Unmarshal(r.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if problem.Code != "RESPONSE_SERIALIZATION_FAILED" {
		t.Fatalf("problem=%#v", problem)
	}
}

func TestInvalidResultStatusReturns500(t *testing.T) {
	var reported error
	r := httptest.NewRecorder()
	JSON(99, struct{}{}).WriteHTTP(r, func(err error) { reported = err })
	if r.Code != http.StatusInternalServerError || reported == nil {
		t.Fatalf("status=%d reported=%v", r.Code, reported)
	}
	if !strings.Contains(reported.Error(), "invalid result status") {
		t.Fatal(reported)
	}
}

type failingWriter struct {
	header http.Header
	status int
}

func (w *failingWriter) Header() http.Header       { return w.header }
func (w *failingWriter) WriteHeader(status int)    { w.status = status }
func (w *failingWriter) Write([]byte) (int, error) { return 0, errors.New("connection closed") }

func TestResultReportsWriteFailure(t *testing.T) {
	writer := &failingWriter{header: make(http.Header)}
	var reported error
	OK(struct{}{}).WriteHTTP(writer, func(err error) { reported = err })
	if writer.status != http.StatusOK || reported == nil {
		t.Fatalf("status=%d reported=%v", writer.status, reported)
	}
}

func TestErrorJSONWritesCallerDefinedBody(t *testing.T) {
	type apiError struct {
		Code string `json:"code"`
	}
	recorder := httptest.NewRecorder()
	ErrorJSON[struct{}](http.StatusConflict, apiError{Code: "CONFLICT"}).write(recorder)
	if recorder.Code != http.StatusConflict || recorder.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("status=%d headers=%v", recorder.Code, recorder.Header())
	}
	if recorder.Header().Get("Cache-Control") != "no-store" || strings.TrimSpace(recorder.Body.String()) != `{"code":"CONFLICT"}` {
		t.Fatalf("headers=%v body=%s", recorder.Header(), recorder.Body.String())
	}
}
