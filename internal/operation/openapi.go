package operation

import (
	"fmt"
	"mime"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/sevlumen/oashttp/v2/internal/binding"
	"github.com/sevlumen/oashttp/v2/internal/oas31"
	"github.com/sevlumen/oashttp/v2/internal/route"
	"github.com/sevlumen/oashttp/v2/internal/schema"
)

func compileOAS(def *Definition, plan *binding.Plan, pattern route.Pattern, registry *schema.Registry, failureContentType string, failureModelType reflect.Type) (*oas31.Operation, error) {
	if registry == nil {
		return nil, fmt.Errorf("schema registry is nil")
	}
	if def.OperationID == "" {
		return nil, fmt.Errorf("operation ID is required")
	}

	var params []binding.ParameterDoc
	var body *binding.BodyDoc
	if plan != nil {
		params, body = plan.Documentation()
	}

	success := false
	responses := map[string]oas31.Response{}
	for status, spec := range def.Responses {
		if status >= 200 && status < 300 {
			success = true
		}
		description := responseDescription(status, spec.Description)
		response := oas31.Response{Description: description}
		var err error
		if statusAllowsResponseBody(status) {
			switch spec.Kind {
			case ResponseProblem:
				response, err = documentedResponse(registry, description, failureContentType, failureModelType)
			case ResponseCustom:
				mediaType, _, _ := mime.ParseMediaType(strings.TrimSpace(spec.ContentType))
				response, err = documentedResponse(registry, description, mediaType, spec.ModelType)
			case ResponseRaw:
				// Raw handlers declare only status/description unless the caller uses ProducesResponse.
			default:
				response, err = documentedResponse(registry, description, "application/json", def.OutputType)
			}
			if err != nil {
				return nil, err
			}
		}
		responses[strconv.Itoa(status)] = response
	}
	if !success {
		return nil, fmt.Errorf("at least one successful response must be declared")
	}

	if plan != nil || rawPathHasValidation(pattern) {
		if err := addFailureResponse(responses, registry, http.StatusBadRequest, failureContentType, failureModelType); err != nil {
			return nil, err
		}
		if body != nil {
			if err := addFailureResponse(responses, registry, http.StatusRequestEntityTooLarge, failureContentType, failureModelType); err != nil {
				return nil, err
			}
			if err := addFailureResponse(responses, registry, http.StatusUnsupportedMediaType, failureContentType, failureModelType); err != nil {
				return nil, err
			}
		}
	}
	if def.SecurityName != "" || def.Feature != "" {
		if err := addFailureResponse(responses, registry, http.StatusUnauthorized, failureContentType, failureModelType); err != nil {
			return nil, err
		}
	}
	if def.Feature != "" {
		if err := addFailureResponse(responses, registry, http.StatusForbidden, failureContentType, failureModelType); err != nil {
			return nil, err
		}
	}
	if err := addFailureResponse(responses, registry, http.StatusInternalServerError, failureContentType, failureModelType); err != nil {
		return nil, err
	}

	operation := &oas31.Operation{
		OperationID: def.OperationID,
		Tags:        append([]string(nil), def.Tags...),
		Summary:     def.Summary,
		Description: def.Description,
		Responses:   responses,
	}

	routeConstraints := map[string]route.Constraint{}
	for _, p := range pattern.Parameters {
		routeConstraints[p.Name] = route.Constraints[p.Constraint]
	}

	if plan != nil {
		for _, p := range params {
			var ref *oas31.Schema
			if p.In == "path" {
				c := routeConstraints[p.Name]
				s := oas31.Schema{"type": c.JSONType}
				if c.Format != "" {
					s["format"] = c.Format
				}
				ref = &s
			} else {
				var err error
				ref, err = registry.Ref(p.Type)
				if err != nil {
					return nil, fmt.Errorf("parameter %s: %w", p.Name, err)
				}
			}
			operation.Parameters = append(operation.Parameters, oas31.Parameter{Name: p.Name, In: p.In, Required: p.Required, Schema: ref})
		}
	} else {
		for _, p := range pattern.Parameters {
			c := routeConstraints[p.Name]
			s := oas31.Schema{"type": c.JSONType}
			if c.Format != "" {
				s["format"] = c.Format
			}
			operation.Parameters = append(operation.Parameters, oas31.Parameter{Name: p.Name, In: "path", Required: true, Schema: &s})
		}
	}

	if body != nil {
		ref, err := registry.Ref(body.Type)
		if err != nil {
			return nil, fmt.Errorf("request body: %w", err)
		}
		operation.RequestBody = &oas31.RequestBody{Required: body.Required, Content: map[string]oas31.MediaType{"application/json": {Schema: ref}}}
	}
	if len(def.RawRequestMediaTypes) > 0 {
		content := make(map[string]oas31.MediaType, len(def.RawRequestMediaTypes))
		for _, raw := range def.RawRequestMediaTypes {
			mediaType, _, _ := mime.ParseMediaType(strings.TrimSpace(raw))
			content[mediaType] = oas31.MediaType{}
		}
		operation.RequestBody = &oas31.RequestBody{Content: content}
	}

	securityName := def.SecurityName
	if securityName == "" && def.Feature != "" {
		securityName = "bearerAuth"
	}
	if securityName != "" {
		operation.Security = []map[string][]string{{securityName: {}}}
	}
	if def.Feature != "" {
		operation.Extensions = map[string]any{"x-feature": def.Feature, "x-permission": def.Permission}
	}
	return operation, nil
}

func rawPathHasValidation(pattern route.Pattern) bool {
	for _, parameter := range pattern.Parameters {
		if parameter.Constraint != "string" {
			return true
		}
	}
	return false
}

func responseDescription(status int, description string) string {
	if description != "" {
		return description
	}
	if text := http.StatusText(status); text != "" {
		return text
	}
	return "Response"
}

func documentedResponse(registry *schema.Registry, description, contentType string, modelType reflect.Type) (oas31.Response, error) {
	ref, err := registry.Ref(modelType)
	if err != nil {
		return oas31.Response{}, err
	}
	return oas31.Response{Description: description, Content: map[string]oas31.MediaType{contentType: {Schema: ref}}}, nil
}

func addFailureResponse(responses map[string]oas31.Response, registry *schema.Registry, status int, contentType string, modelType reflect.Type) error {
	key := strconv.Itoa(status)
	if _, exists := responses[key]; exists {
		return nil
	}
	response, err := documentedResponse(registry, responseDescription(status, ""), contentType, modelType)
	if err != nil {
		return err
	}
	responses[key] = response
	return nil
}
