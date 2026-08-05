package oashttp

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResultWritesJSONSuccess(t *testing.T) {
	r := httptest.NewRecorder()
	OK(struct {
		ID string `json:"id"`
	}{"u-1"}).write(r)
	if r.Code != http.StatusOK {
		t.Fatalf("status=%d", r.Code)
	}
	if got := r.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type=%q", got)
	}
}
func TestProblemWritesProblemJSON(t *testing.T) {
	r := httptest.NewRecorder()
	NotFound[struct{}]("USER_NOT_FOUND", "User was not found").write(r)
	if r.Code != http.StatusNotFound {
		t.Fatalf("status=%d", r.Code)
	}
	if got := r.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Fatalf("Content-Type=%q", got)
	}
}
