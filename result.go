package oashttp

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/quang020102/go-osm/internal/core"
	internalfailure "github.com/quang020102/go-osm/internal/failure"
)

type Result[T any] struct {
	status      int
	headers     http.Header
	body        any
	contentType string
	failure     *Failure
	noStore     bool
}

func OK[T any](value T) Result[T]       { return JSON(http.StatusOK, value) }
func Created[T any](value T) Result[T]  { return JSON(http.StatusCreated, value) }
func Accepted[T any](value T) Result[T] { return JSON(http.StatusAccepted, value) }

// JSON creates a JSON response with an explicit HTTP status code.
func JSON[T any](status int, value T) Result[T] {
	return Result[T]{status: status, headers: make(http.Header), body: value, contentType: "application/json"}
}

// ErrorJSON creates an application/json error response with a caller-defined body.
// Declare its OpenAPI schema with ProducesResponse.
func ErrorJSON[T any, E any](status int, value E) Result[T] {
	return Result[T]{status: status, headers: make(http.Header), body: value, contentType: "application/json", noStore: true}
}

func NoContent[T any]() Result[T] {
	return Result[T]{status: http.StatusNoContent, headers: make(http.Header)}
}

func (r Result[T]) WithHeader(name, value string) Result[T] {
	if r.headers == nil {
		r.headers = make(http.Header)
	}
	r.headers.Add(name, value)
	return r
}

// WriteHTTP writes the result using the default Problem Details formatter.
// Applications built through App automatically use Config.FailureFormatter.
func (r Result[T]) WriteHTTP(w http.ResponseWriter, onError func(error)) {
	r.WriteHTTPWithFailureFormatter(w, onError, ProblemDetailsFormatter{}, "application/problem+json")
}

func (r Result[T]) WriteHTTPWithFailureFormatter(w http.ResponseWriter, onError func(error), formatter core.FailureFormatter, failureContentType string) {
	status := r.status
	if status == 0 {
		status = http.StatusOK
	}
	if status < 100 || status > 599 {
		reportWriteError(onError, fmt.Errorf("oashttp: invalid result status %d", status))
		internalfailure.WriteResolved(w, formatter, failureContentType, core.Failure{
			Status: http.StatusInternalServerError,
			Code:   "RESPONSE_SERIALIZATION_FAILED",
			Detail: "The response could not be serialized",
		}, onError)
		return
	}

	for key, values := range r.headers {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	if r.failure != nil {
		failure := *r.failure
		if failure.Status == 0 {
			failure.Status = status
		}
		internalfailure.WriteResolved(w, formatter, failureContentType, failure, onError)
		return
	}

	var payload []byte
	if statusAllowsBody(status) && r.body != nil {
		var err error
		payload, err = json.Marshal(r.body)
		if err != nil {
			reportWriteError(onError, fmt.Errorf("oashttp: encode response: %w", err))
			internalfailure.WriteResolved(w, formatter, failureContentType, core.Failure{
				Status: http.StatusInternalServerError,
				Code:   "RESPONSE_SERIALIZATION_FAILED",
				Detail: "The response could not be serialized",
			}, onError)
			return
		}
	}

	if r.contentType != "" && statusAllowsBody(status) {
		w.Header().Set("Content-Type", r.contentType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
	}
	if r.noStore {
		w.Header().Set("Cache-Control", "no-store")
	}
	w.WriteHeader(status)
	if len(payload) != 0 {
		if _, err := w.Write(payload); err != nil {
			reportWriteError(onError, fmt.Errorf("oashttp: write response: %w", err))
		}
	}
}

func statusAllowsBody(status int) bool {
	return status >= 200 && status != http.StatusNoContent && status != http.StatusNotModified
}

func (r Result[T]) write(w http.ResponseWriter) { r.WriteHTTP(w, nil) }
func reportWriteError(handler func(error), err error) {
	if handler != nil {
		handler(err)
	} else {
		log.Printf("oashttp: %v", err)
	}
}

var _ core.ResultWriter = Result[struct{}]{}
