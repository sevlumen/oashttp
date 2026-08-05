package oashttp

import (
	"net/http"

	"github.com/quang020102/go-osm/internal/core"
)

type ProblemDetails = core.ProblemDetails

func BadRequest[T any](code, detail string) Result[T] {
	return problemResult[T](http.StatusBadRequest, "Bad Request", code, detail)
}
func Unauthorized[T any](code, detail string) Result[T] {
	return problemResult[T](http.StatusUnauthorized, "Unauthorized", code, detail)
}
func Forbidden[T any](code, detail string) Result[T] {
	return problemResult[T](http.StatusForbidden, "Forbidden", code, detail)
}
func NotFound[T any](code, detail string) Result[T] {
	return problemResult[T](http.StatusNotFound, "Not Found", code, detail)
}
func Conflict[T any](code, detail string) Result[T] {
	return problemResult[T](http.StatusConflict, "Conflict", code, detail)
}
func InternalError[T any](code, detail string) Result[T] {
	return problemResult[T](http.StatusInternalServerError, "Internal Server Error", code, detail)
}

func problemResult[T any](status int, title, code, detail string) Result[T] {
	p := &ProblemDetails{Title: title, Status: status, Code: code, Detail: detail}
	return Result[T]{status: status, headers: make(http.Header), problem: p}
}
