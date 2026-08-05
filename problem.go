package oashttp

import (
	"net/http"

	"github.com/quang020102/go-osm/internal/core"
)

type ProblemDetails = core.ProblemDetails
type Failure = core.Failure
type FailureFormatter = core.FailureFormatter
type ProblemDetailsFormatter = core.ProblemDetailsFormatter

// Fail creates a result whose public body is produced by Config.FailureFormatter.
func Fail[T any](failure Failure) Result[T] {
	return Result[T]{status: failure.Status, headers: make(http.Header), failure: &failure}
}

func BadRequest[T any](code, detail string) Result[T] {
	return failureResult[T](http.StatusBadRequest, "Bad Request", code, detail)
}
func Unauthorized[T any](code, detail string) Result[T] {
	return failureResult[T](http.StatusUnauthorized, "Unauthorized", code, detail)
}
func Forbidden[T any](code, detail string) Result[T] {
	return failureResult[T](http.StatusForbidden, "Forbidden", code, detail)
}
func NotFound[T any](code, detail string) Result[T] {
	return failureResult[T](http.StatusNotFound, "Not Found", code, detail)
}
func Conflict[T any](code, detail string) Result[T] {
	return failureResult[T](http.StatusConflict, "Conflict", code, detail)
}
func InternalError[T any](code, detail string) Result[T] {
	return failureResult[T](http.StatusInternalServerError, "Internal Server Error", code, detail)
}

func failureResult[T any](status int, title, code, detail string) Result[T] {
	return Fail[T](Failure{Title: title, Status: status, Code: code, Detail: detail})
}
