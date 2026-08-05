package binding

import "fmt"

// RequestError describes a request-level binding failure that maps to a
// specific HTTP status. Field-level conversion and validation failures remain
// represented by core.FieldError.
type RequestError struct {
	Status int
	Code   string
	Detail string
}

func (e *RequestError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Detail)
}
