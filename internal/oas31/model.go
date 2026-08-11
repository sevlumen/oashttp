package oas31

import (
	"encoding/json"
	"fmt"
)

type Document struct {
	OpenAPI           string               `json:"openapi"`
	JSONSchemaDialect string               `json:"jsonSchemaDialect,omitempty"`
	Info              Info                 `json:"info"`
	Servers           []Server             `json:"servers,omitempty"`
	Paths             map[string]*PathItem `json:"paths"`
	Components        Components           `json:"components,omitempty"`
}
type Info struct {
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
}
type Server struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}
type Components struct {
	Schemas         map[string]*Schema        `json:"schemas,omitempty"`
	Responses       map[string]Response       `json:"responses,omitempty"`
	SecuritySchemes map[string]SecurityScheme `json:"securitySchemes,omitempty"`
}
type PathItem struct {
	Get     *Operation `json:"get,omitempty"`
	Post    *Operation `json:"post,omitempty"`
	Put     *Operation `json:"put,omitempty"`
	Patch   *Operation `json:"patch,omitempty"`
	Delete  *Operation `json:"delete,omitempty"`
	Options *Operation `json:"options,omitempty"`
	Head    *Operation `json:"head,omitempty"`
	Trace   *Operation `json:"trace,omitempty"`
}
type Operation struct {
	OperationID string                `json:"operationId,omitempty"`
	Tags        []string              `json:"tags,omitempty"`
	Summary     string                `json:"summary,omitempty"`
	Description string                `json:"description,omitempty"`
	Parameters  []Parameter           `json:"parameters,omitempty"`
	RequestBody *RequestBody          `json:"requestBody,omitempty"`
	Responses   map[string]Response   `json:"responses"`
	Security    []map[string][]string `json:"security,omitempty"`
	Extensions  map[string]any        `json:"-"`
}
type Parameter struct {
	Name        string  `json:"name"`
	In          string  `json:"in"`
	Description string  `json:"description,omitempty"`
	Required    bool    `json:"required,omitempty"`
	Schema      *Schema `json:"schema,omitempty"`
}
type RequestBody struct {
	Required bool                 `json:"required,omitempty"`
	Content  map[string]MediaType `json:"content"`
}
type MediaType struct {
	Schema *Schema `json:"schema,omitempty"`
}
type Response struct {
	Ref         string               `json:"$ref,omitempty"`
	Description string               `json:"description,omitempty"`
	Content     map[string]MediaType `json:"content,omitempty"`
}
type SecurityScheme struct {
	Type         string `json:"type"`
	Scheme       string `json:"scheme,omitempty"`
	BearerFormat string `json:"bearerFormat,omitempty"`
	Name         string `json:"name,omitempty"`
	In           string `json:"in,omitempty"`
}
type Schema map[string]any

func (o Operation) MarshalJSON() ([]byte, error) {
	type alias Operation
	base, err := json.Marshal(alias(o))
	if err != nil {
		return nil, err
	}
	var object map[string]any
	if err := json.Unmarshal(base, &object); err != nil {
		return nil, err
	}
	for key, value := range o.Extensions {
		if len(key) < 3 || key[:2] != "x-" {
			return nil, fmt.Errorf("invalid OpenAPI extension %q", key)
		}
		object[key] = value
	}
	return json.Marshal(object)
}
