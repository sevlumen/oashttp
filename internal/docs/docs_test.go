package docs

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAPIHandlerServesImmutableJSONWithETag(t *testing.T) {
	h := NewOpenAPIHandler([]byte(`{"openapi":"3.1.0"}`))
	r := httptest.NewRecorder()
	h.ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	if r.Code != http.StatusOK || r.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("status=%d headers=%v", r.Code, r.Header())
	}
	etag := r.Header().Get("ETag")
	if etag == "" {
		t.Fatal("missing ETag")
	}
	conditional := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	conditional.Header.Set("If-None-Match", etag)
	notModified := httptest.NewRecorder()
	h.ServeHTTP(notModified, conditional)
	if notModified.Code != http.StatusNotModified || notModified.Body.Len() != 0 {
		t.Fatalf("status=%d body=%q", notModified.Code, notModified.Body.String())
	}
}

func TestOpenAPIHandlerSupportsHeadAndRejectsOtherMethods(t *testing.T) {
	h := NewOpenAPIHandler([]byte(`{}`))
	head := httptest.NewRecorder()
	h.ServeHTTP(head, httptest.NewRequest(http.MethodHead, "/openapi.json", nil))
	if head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Fatalf("status=%d body=%q", head.Code, head.Body.String())
	}
	post := httptest.NewRecorder()
	h.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/openapi.json", nil))
	if post.Code != http.StatusMethodNotAllowed || post.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("status=%d headers=%v", post.Code, post.Header())
	}
}

func TestSwaggerHTMLUsesPinnedCDNAndHashedInlineScript(t *testing.T) {
	h, err := NewSwaggerHandler(Config{Title: "Users API", DocumentURL: "/openapi.json", CDNVersion: "5.32.11"})
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRecorder()
	h.ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/swagger", nil))
	body := r.Body.String()
	for _, expected := range []string{"swagger-ui-dist@5.32.11/swagger-ui.css", "swagger-ui-dist@5.32.11/swagger-ui-bundle.js", `url: "/openapi.json"`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("missing %q in %s", expected, body)
		}
	}
	csp := r.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "'sha256-") || strings.Contains(csp, "'unsafe-inline'") {
		t.Fatalf("CSP=%q", csp)
	}
}

func TestSwaggerRejectsCrossOriginAndMalformedDocumentURLs(t *testing.T) {
	for _, raw := range []string{"https://example.com/openapi.json", "//example.com/openapi.json", `/\\evil`, "/openapi.json\nnext"} {
		if _, err := NewSwaggerHandler(Config{DocumentURL: raw}); err == nil {
			t.Fatalf("DocumentURL %q accepted", raw)
		}
	}
}

func TestSwaggerSupportsHeadAndRejectsPost(t *testing.T) {
	h, err := NewSwaggerHandler(Config{DocumentURL: "/openapi.json"})
	if err != nil {
		t.Fatal(err)
	}
	head := httptest.NewRecorder()
	h.ServeHTTP(head, httptest.NewRequest(http.MethodHead, "/swagger", nil))
	if head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Fatalf("status=%d body=%q", head.Code, head.Body.String())
	}
	post := httptest.NewRecorder()
	h.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/swagger", nil))
	if post.Code != http.StatusMethodNotAllowed || post.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("status=%d headers=%v", post.Code, post.Header())
	}
}
