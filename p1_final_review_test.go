package oashttp

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type finalReviewSecurityProvider struct {
	scheme SecurityScheme
}

func (p finalReviewSecurityProvider) SecurityScheme() SecurityScheme { return p.scheme }
func (finalReviewSecurityProvider) Authenticate(context.Context, *http.Request) (*Principal, error) {
	return &Principal{Subject: "client"}, nil
}

func TestRawConsumesDoesNotClaimRequestBodyIsRequired(t *testing.T) {
	app := New(Config{Info: Info{Title: "Raw optional body", Version: "1.0.0"}})
	MapHandler(app.Group(""), http.MethodPost, "/upload", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).WithOperationID("optionalRawBody").Consumes("application/octet-stream").Produces(http.StatusNoContent)
	if err := app.MapOpenAPI("/openapi.json"); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	app.MustBuild().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	body := recorder.Body.String()
	if !strings.Contains(body, `"application/octet-stream":{}`) {
		t.Fatalf("OpenAPI missing raw media type: %s", body)
	}
	if strings.Contains(body, `"required":true`) {
		t.Fatalf("Consumes must not make an application-owned raw body mandatory: %s", body)
	}
}

func TestNamedSecurityProviderRejectsInvalidOpenAPIComponentName(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("expected invalid security component name to panic")
		}
	}()

	app := New(Config{
		Info: Info{Title: "Invalid security name", Version: "1.0.0"},
		SecurityProviders: map[string]SecurityProvider{
			"bad name": finalReviewSecurityProvider{scheme: SecurityScheme{Type: "apiKey", Name: "X-API-Key", In: "header"}},
		},
	})
	MapGet(app.Group(""), "/protected", func(context.Context, struct{}) Result[map[string]bool] {
		return OK(map[string]bool{"ok": true})
	}).WithOperationID("invalidSecurityName").RequireSecurity("bad name").Produces(http.StatusOK)
}

func TestNamedSecurityProviderRejectsFieldsForAnotherSchemeType(t *testing.T) {
	cases := []struct {
		name   string
		scheme SecurityScheme
	}{
		{
			name:   "http with api key fields",
			scheme: SecurityScheme{Type: "http", Scheme: "bearer", Name: "X-API-Key", In: "header"},
		},
		{
			name:   "api key with http fields",
			scheme: SecurityScheme{Type: "apiKey", Name: "X-API-Key", In: "header", Scheme: "bearer", BearerFormat: "JWT"},
		},
		{
			name:   "non bearer with bearer format",
			scheme: SecurityScheme{Type: "http", Scheme: "basic", BearerFormat: "JWT"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := New(Config{
				Info: Info{Title: "Invalid security scheme", Version: "1.0.0"},
				SecurityProviders: map[string]SecurityProvider{
					"clientAuth": finalReviewSecurityProvider{scheme: tc.scheme},
				},
			})
			MapGet(app.Group(""), "/protected", func(context.Context, struct{}) Result[map[string]bool] {
				return OK(map[string]bool{"ok": true})
			}).WithOperationID("invalidSecurityScheme").RequireSecurity("clientAuth").Produces(http.StatusOK)
			if _, err := app.Build(); err == nil {
				t.Fatal("expected invalid OpenAPI security scheme metadata to fail Build")
			}
		})
	}
}

func TestRawHandlerSupportsAllOpenAPIHTTPMethods(t *testing.T) {
	app := New(Config{Info: Info{Title: "Raw methods", Version: "1.0.0"}})
	methods := []struct {
		method string
		path   string
		id     string
		key    string
	}{
		{method: http.MethodHead, path: "/head", id: "rawHead", key: `"head":`},
		{method: http.MethodOptions, path: "/options", id: "rawOptions", key: `"options":`},
		{method: http.MethodTrace, path: "/trace", id: "rawTrace", key: `"trace":`},
	}
	for _, item := range methods {
		MapHandler(app.Group(""), item.method, item.path, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})).WithOperationID(item.id).Produces(http.StatusNoContent)
	}
	if err := app.MapOpenAPI("/openapi.json"); err != nil {
		t.Fatal(err)
	}
	handler := app.MustBuild()

	for _, item := range methods {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(item.method, item.path, nil))
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("%s %s status=%d", item.method, item.path, recorder.Code)
		}
	}

	doc := httptest.NewRecorder()
	handler.ServeHTTP(doc, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	body := doc.Body.String()
	for _, item := range methods {
		if !strings.Contains(body, item.key) || !strings.Contains(body, `"operationId":"`+item.id+`"`) {
			t.Fatalf("OpenAPI missing %s operation %s: %s", item.method, item.id, body)
		}
	}
}

type finalReviewCapabilityWriter struct {
	*httptest.ResponseRecorder
}

func (*finalReviewCapabilityWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, errors.New("not used")
}

func TestRawHandlerPreservesFlusherAndHijackerThroughRecovery(t *testing.T) {
	app := New(Config{Info: Info{Title: "Raw capabilities", Version: "1.0.0"}})
	MapHandler(app.Group(""), http.MethodGet, "/capabilities", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, ok := w.(http.Flusher); !ok {
			t.Error("raw handler lost http.Flusher through default recovery")
		}
		if _, ok := w.(http.Hijacker); !ok {
			t.Error("raw handler lost http.Hijacker through default recovery")
		}
		w.WriteHeader(http.StatusNoContent)
	})).WithOperationID("rawCapabilities").Produces(http.StatusNoContent)

	writer := &finalReviewCapabilityWriter{ResponseRecorder: httptest.NewRecorder()}
	app.MustBuild().ServeHTTP(writer, httptest.NewRequest(http.MethodGet, "/capabilities", nil))
	if writer.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", writer.Code, writer.Body.String())
	}
}
