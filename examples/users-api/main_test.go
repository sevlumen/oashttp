package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/sevlumen/oashttp/v2"
)

type testAuthenticator struct{}

func (testAuthenticator) Authenticate(_ context.Context, token string) (*oashttp.Principal, error) {
	switch token {
	case "allowed":
		return &oashttp.Principal{Subject: "u-1", Features: map[string]struct{}{"core.users": {}}, Permissions: map[string]struct{}{"core.users.update": {}}}, nil
	case "no-permission":
		return &oashttp.Principal{Subject: "u-2", Features: map[string]struct{}{"core.users": {}}, Permissions: map[string]struct{}{}}, nil
	default:
		return nil, errors.New("invalid token")
	}
}

func TestUsersAPIEndToEnd(t *testing.T) {
	handler, err := newUsersApp(testAuthenticator{}).Build()
	if err != nil {
		t.Fatal(err)
	}
	t.Run("valid update", func(t *testing.T) {
		r := request(t, handler, http.MethodPut, "/api/v1/core/users/550e8400-e29b-41d4-a716-446655440000", `{"fullName":"Alice","email":"alice@example.com"}`, "allowed")
		if r.Code != 200 {
			t.Fatalf("status=%d body=%s", r.Code, r.Body.String())
		}
	})
	t.Run("invalid UUID", func(t *testing.T) {
		r := request(t, handler, http.MethodPut, "/api/v1/core/users/not-a-uuid", `{"fullName":"Alice","email":"alice@example.com"}`, "allowed")
		if r.Code != 400 || r.Header().Get("Content-Type") != "application/problem+json" {
			t.Fatalf("status=%d body=%s", r.Code, r.Body.String())
		}
	})
	t.Run("invalid body", func(t *testing.T) {
		r := request(t, handler, http.MethodPut, "/api/v1/core/users/550e8400-e29b-41d4-a716-446655440000", `{"fullName":"x","email":"bad"}`, "allowed")
		if r.Code != 400 || !strings.Contains(r.Body.String(), "body.fullName") {
			t.Fatalf("status=%d body=%s", r.Code, r.Body.String())
		}
	})
	t.Run("missing bearer", func(t *testing.T) {
		r := request(t, handler, http.MethodPut, "/api/v1/core/users/550e8400-e29b-41d4-a716-446655440000", `{"fullName":"Alice","email":"alice@example.com"}`, "")
		if r.Code != 401 {
			t.Fatalf("status=%d body=%s", r.Code, r.Body.String())
		}
	})
	t.Run("missing permission", func(t *testing.T) {
		r := request(t, handler, http.MethodPut, "/api/v1/core/users/550e8400-e29b-41d4-a716-446655440000", `{"fullName":"Alice","email":"alice@example.com"}`, "no-permission")
		if r.Code != 403 {
			t.Fatalf("status=%d body=%s", r.Code, r.Body.String())
		}
	})
	t.Run("openapi golden", func(t *testing.T) {
		r := request(t, handler, http.MethodGet, "/openapi.json", "", "")
		if r.Code != 200 {
			t.Fatalf("status=%d", r.Code)
		}
		actual := normalizeJSON(t, r.Body.Bytes())
		if os.Getenv("UPDATE_GOLDEN") == "1" {
			if err := os.WriteFile("../../testdata/users.openapi.golden.json", append(actual, '\n'), 0o644); err != nil {
				t.Fatal(err)
			}
			return
		}
		golden, err := os.ReadFile("../../testdata/users.openapi.golden.json")
		if err != nil {
			t.Fatal(err)
		}
		expected := normalizeJSON(t, golden)
		if string(actual) != string(expected) {
			t.Fatalf("OpenAPI mismatch\nactual:\n%s\nexpected:\n%s", actual, expected)
		}
	})
	t.Run("swagger", func(t *testing.T) {
		r := request(t, handler, http.MethodGet, "/swagger", "", "")
		body := r.Body.String()
		if r.Code != 200 || !strings.Contains(body, "swagger-ui-dist@5.32.11") || !strings.Contains(body, "/openapi.json") {
			t.Fatalf("status=%d body=%s", r.Code, body)
		}
	})
}
func request(t *testing.T, handler http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}
func normalizeJSON(t *testing.T, data []byte) []byte {
	t.Helper()
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, data)
	}
	normalized, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return normalized
}
