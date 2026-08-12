package oashttp

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResetContentDoesNotWritePayloadOrRepresentationHeaders(t *testing.T) {
	recorder := httptest.NewRecorder()
	JSON(http.StatusResetContent, struct {
		Message string `json:"message"`
	}{Message: "must not be written"}).write(recorder)

	if recorder.Code != http.StatusResetContent {
		t.Fatalf("status=%d", recorder.Code)
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("body=%q", recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != "" {
		t.Fatalf("Content-Type=%q", got)
	}
	if got := recorder.Header().Get("X-Content-Type-Options"); got != "" {
		t.Fatalf("X-Content-Type-Options=%q", got)
	}
}
