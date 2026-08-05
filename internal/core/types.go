package core

import (
	"context"
	"net/http"
)

type Server struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

type FieldError struct {
	Location string   `json:"location"`
	Field    string   `json:"field"`
	Messages []string `json:"messages"`
}

type ResultWriter interface {
	WriteHTTP(http.ResponseWriter, func(error))
}

type Validator interface {
	Validate(context.Context, any) []FieldError
}
