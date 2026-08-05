package failure

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"reflect"
	"strings"

	"github.com/quang020102/go-osm/internal/core"
)

// Describe validates a formatter and returns the media type and model type used
// to document framework-generated failures in OpenAPI.
func Describe(formatter core.FailureFormatter) (contentType string, modelType reflect.Type, err error) {
	if formatter == nil {
		formatter = core.ProblemDetailsFormatter{}
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			contentType = ""
			modelType = nil
			err = fmt.Errorf("failure formatter panicked: %v", recovered)
		}
	}()

	contentType = strings.TrimSpace(formatter.ContentType())
	if contentType == "" {
		return "", nil, fmt.Errorf("failure formatter content type is required")
	}
	mediaType, _, parseErr := mime.ParseMediaType(contentType)
	if parseErr != nil {
		return "", nil, fmt.Errorf("invalid failure formatter content type %q: %w", contentType, parseErr)
	}
	model := formatter.Model()
	if model == nil {
		return "", nil, fmt.Errorf("failure formatter model is required")
	}
	return mediaType, reflect.TypeOf(model), nil
}

// Write formats and serializes a failure before committing response headers.
// If the custom formatter fails, a minimal Problem Details response is used.
func Write(w http.ResponseWriter, formatter core.FailureFormatter, item core.Failure, onError func(error)) {
	contentType, _, err := Describe(formatter)
	if err != nil {
		writeFallback(w, err, onError)
		return
	}
	WriteResolved(w, formatter, contentType, item, onError)
}

// WriteResolved uses a media type validated during application build, keeping
// runtime responses aligned with the generated OpenAPI document.
func WriteResolved(w http.ResponseWriter, formatter core.FailureFormatter, contentType string, item core.Failure, onError func(error)) {
	if formatter == nil {
		formatter = core.ProblemDetailsFormatter{}
	}
	item = normalize(item)
	body, err := format(formatter, item)
	if err == nil && body == nil {
		err = fmt.Errorf("failure formatter returned a nil body")
	}
	if err == nil {
		var payload []byte
		payload, err = json.Marshal(body)
		if err == nil {
			write(w, item.Status, contentType, payload, onError)
			return
		}
	}
	writeFallback(w, err, onError)
}

func writeFallback(w http.ResponseWriter, err error, onError func(error)) {
	report(onError, fmt.Errorf("oashttp: format failure response: %w", err))
	fallback := core.ProblemDetails{
		Title:  http.StatusText(http.StatusInternalServerError),
		Status: http.StatusInternalServerError,
		Code:   "FAILURE_FORMATTING_FAILED",
		Detail: "The error response could not be formatted",
	}
	payload, _ := json.Marshal(fallback)
	write(w, http.StatusInternalServerError, "application/problem+json", payload, onError)
}

func normalize(item core.Failure) core.Failure {
	if item.Status < 400 || item.Status > 599 {
		item.Status = http.StatusInternalServerError
		item.Code = "INVALID_FAILURE_STATUS"
		item.Detail = "The failure status was invalid"
	}
	if strings.TrimSpace(item.Title) == "" {
		item.Title = http.StatusText(item.Status)
		if item.Title == "" {
			item.Title = "Error"
		}
	}
	return item
}

func format(formatter core.FailureFormatter, item core.Failure) (body any, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			body = nil
			err = fmt.Errorf("failure formatter panicked: %v", recovered)
		}
	}()
	return formatter.Format(item), nil
}

func write(w http.ResponseWriter, status int, contentType string, payload []byte, onError func(error)) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if len(payload) == 0 {
		return
	}
	if _, err := w.Write(payload); err != nil {
		report(onError, fmt.Errorf("oashttp: write failure response: %w", err))
	}
}

func report(handler func(error), err error) {
	if handler != nil && err != nil {
		handler(err)
	}
}
