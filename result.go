package oashttp

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/quang020102/go-osm/internal/core"
)

type Result[T any] struct {
	status  int
	headers http.Header
	value   *T
	problem *ProblemDetails
}

func OK[T any](value T) Result[T]       { return JSON(http.StatusOK, value) }
func Created[T any](value T) Result[T]  { return JSON(http.StatusCreated, value) }
func Accepted[T any](value T) Result[T] { return JSON(http.StatusAccepted, value) }

// JSON creates a JSON response with an explicit HTTP status code.
func JSON[T any](status int, value T) Result[T] {
	return Result[T]{status: status, headers: make(http.Header), value: &value}
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

func (r Result[T]) WriteHTTP(w http.ResponseWriter, onError func(error)) {
	status := r.status
	if status == 0 {
		status = http.StatusOK
	}
	if status < 100 || status > 599 {
		err := fmt.Errorf("oashttp: invalid result status %d", status)
		reportWriteError(onError, err)
		writeSerializationProblem(w)
		return
	}

	var (
		payload     []byte
		contentType string
		err         error
	)

	switch {
	case r.problem != nil:
		contentType = "application/problem+json"
		payload, err = json.Marshal(r.problem)
	case !statusAllowsBody(status):
		// RFC 9110 disallows payload bodies on 1xx, 204, and 304 responses.
	case r.value != nil:
		contentType = "application/json"
		payload, err = json.Marshal(r.value)
	}

	if err != nil {
		reportWriteError(onError, fmt.Errorf("oashttp: encode response: %w", err))
		writeSerializationProblem(w)
		return
	}

	for key, values := range r.headers {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
	}
	if r.problem != nil {
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

func writeSerializationProblem(w http.ResponseWriter) {
	payload, _ := json.Marshal(core.ProblemDetails{
		Title:  http.StatusText(http.StatusInternalServerError),
		Status: http.StatusInternalServerError,
		Code:   "RESPONSE_SERIALIZATION_FAILED",
		Detail: "The response could not be serialized",
	})
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write(payload)
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
