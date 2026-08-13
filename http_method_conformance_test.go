package oashttp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func buildMethodConformanceApp(t *testing.T) http.Handler {
	t.Helper()
	app := New(Config{Info: Info{Title: "HTTP method conformance", Version: "test"}})
	group := app.Group("")

	MapGet(group, "/items", func(context.Context, struct{}) Result[map[string]string] {
		return OK(map[string]string{"method": "get"})
	}).WithOperationID("getItems").Produces(http.StatusOK)

	MapPost(group, "/items", func(context.Context, struct{}) Result[map[string]string] {
		return Created(map[string]string{"method": "post"})
	}).WithOperationID("createItem").Produces(http.StatusCreated)

	MapHandler(group, http.MethodOptions, "/explicit-options", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Allow", "GET, HEAD, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
	})).WithOperationID("explicitOptions").Produces(http.StatusNoContent)

	MapGet(group, "/slash/", func(context.Context, struct{}) Result[struct{}] {
		return NoContent[struct{}]()
	}).WithOperationID("slashRoute").Produces(http.StatusNoContent)

	return app.MustBuild()
}

func TestHTTPMethodConformanceGETAlsoServesHEAD(t *testing.T) {
	server := httptest.NewServer(buildMethodConformanceApp(t))
	defer server.Close()

	req, err := http.NewRequest(http.MethodHead, server.URL+"/items", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if len(body) != 0 {
		t.Fatalf("HEAD body=%q", body)
	}
	if got := resp.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type=%q", got)
	}
}

func TestHTTPMethodConformanceMethodMismatchUsesServeMuxAllow(t *testing.T) {
	handler := buildMethodConformanceApp(t)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/items", nil))

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Allow"); got != "GET, HEAD, POST" {
		t.Fatalf("Allow=%q", got)
	}
}

func TestHTTPMethodConformanceOPTIONSIsNotSynthesized(t *testing.T) {
	handler := buildMethodConformanceApp(t)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodOptions, "/items", nil))

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Allow"); got != "GET, HEAD, POST" {
		t.Fatalf("Allow=%q", got)
	}
}

func TestHTTPMethodConformanceExplicitOPTIONSRunsRawHandler(t *testing.T) {
	handler := buildMethodConformanceApp(t)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodOptions, "/explicit-options", nil))

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status=%d", recorder.Code)
	}
	if got := recorder.Header().Get("Allow"); got != "GET, HEAD, OPTIONS" {
		t.Fatalf("Allow=%q", got)
	}
}

func TestHTTPMethodConformanceTrailingSlashIsExactButCanonical(t *testing.T) {
	handler := buildMethodConformanceApp(t)

	exact := httptest.NewRecorder()
	handler.ServeHTTP(exact, httptest.NewRequest(http.MethodGet, "/slash/", nil))
	if exact.Code != http.StatusNoContent {
		t.Fatalf("exact status=%d", exact.Code)
	}

	descendant := httptest.NewRecorder()
	handler.ServeHTTP(descendant, httptest.NewRequest(http.MethodGet, "/slash/child", nil))
	if descendant.Code != http.StatusNotFound {
		t.Fatalf("descendant status=%d", descendant.Code)
	}

	missingSlash := httptest.NewRecorder()
	handler.ServeHTTP(missingSlash, httptest.NewRequest(http.MethodGet, "/slash", nil))
	switch missingSlash.Code {
	case http.StatusMovedPermanently, http.StatusTemporaryRedirect:
	default:
		t.Fatalf("redirect status=%d", missingSlash.Code)
	}
	if got := missingSlash.Header().Get("Location"); got != "/slash/" {
		t.Fatalf("Location=%q", got)
	}
}
