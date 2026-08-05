package oashttp

import (
	"encoding/json"
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

func OK[T any](value T) Result[T]       { return success(http.StatusOK, value) }
func Created[T any](value T) Result[T]  { return success(http.StatusCreated, value) }
func Accepted[T any](value T) Result[T] { return success(http.StatusAccepted, value) }
func NoContent[T any]() Result[T] {
	return Result[T]{status: http.StatusNoContent, headers: make(http.Header)}
}

func success[T any](status int, value T) Result[T] {
	return Result[T]{status: status, headers: make(http.Header), value: &value}
}

func (r Result[T]) WithHeader(name, value string) Result[T] {
	if r.headers == nil {
		r.headers = make(http.Header)
	}
	r.headers.Add(name, value)
	return r
}

func (r Result[T]) WriteHTTP(w http.ResponseWriter, onError func(error)) {
	for key, values := range r.headers {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	status := r.status
	if status == 0 {
		status = http.StatusOK
	}
	if r.problem != nil {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(status)
		if err := json.NewEncoder(w).Encode(r.problem); err != nil {
			reportWriteError(onError, err)
		}
		return
	}
	if status == http.StatusNoContent {
		w.WriteHeader(status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if r.value != nil {
		if err := json.NewEncoder(w).Encode(r.value); err != nil {
			reportWriteError(onError, err)
		}
	}
}

func (r Result[T]) write(w http.ResponseWriter) { r.WriteHTTP(w, nil) }
func reportWriteError(handler func(error), err error) {
	if handler != nil {
		handler(err)
	} else {
		log.Printf("oashttp: encode response: %v", err)
	}
}

var _ core.ResultWriter = Result[struct{}]{}
