package docs

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAPIHandlerServesImmutableJSON(t *testing.T) {
	h := NewOpenAPIHandler([]byte(`{"openapi":"3.1.0"}`))
	r := httptest.NewRecorder()
	h.ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	if r.Code != 200 || r.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("status=%d headers=%v", r.Code, r.Header())
	}
}
func TestSwaggerHTMLUsesPinnedCDNAndRelativeDocumentURL(t *testing.T) {
	h, e := NewSwaggerHandler(Config{Title: "Users API", DocumentURL: "/openapi.json", CDNVersion: "5.32.11"})
	if e != nil {
		t.Fatal(e)
	}
	r := httptest.NewRecorder()
	h.ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/swagger", nil))
	body := r.Body.String()
	for _, s := range []string{"swagger-ui-dist@5.32.11/swagger-ui.css", "swagger-ui-dist@5.32.11/swagger-ui-bundle.js", `url: "/openapi.json"`} {
		if !strings.Contains(body, s) {
			t.Fatalf("missing %q in %s", s, body)
		}
	}
}
