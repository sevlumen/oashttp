package oashttp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPBodyConformanceClientSeesNoPayloadForBodylessStatuses(t *testing.T) {
	cases := []struct {
		name   string
		status int
	}{
		{name: "no-content", status: http.StatusNoContent},
		{name: "reset-content", status: http.StatusResetContent},
		{name: "not-modified", status: http.StatusNotModified},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := New(Config{Info: Info{Title: "Body conformance", Version: "test"}})
			MapGet(app.Group(""), "/status", func(context.Context, struct{}) Result[map[string]string] {
				return JSON(tc.status, map[string]string{"must": "not appear"})
			}).WithOperationID("bodylessStatus").Produces(http.StatusOK).Produces(tc.status)

			server := httptest.NewServer(app.MustBuild())
			defer server.Close()

			resp, err := server.Client().Get(server.URL + "/status")
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}

			if resp.StatusCode != tc.status {
				t.Fatalf("status=%d", resp.StatusCode)
			}
			if len(body) != 0 {
				t.Fatalf("body=%q", body)
			}
			if got := resp.Header.Get("Content-Type"); got != "" {
				t.Fatalf("Content-Type=%q", got)
			}
		})
	}
}

func TestHTTPBodyConformanceHEADSuppressesSerializedPayloadOnWire(t *testing.T) {
	app := New(Config{Info: Info{Title: "HEAD conformance", Version: "test"}})
	MapGet(app.Group(""), "/head", func(context.Context, struct{}) Result[map[string]string] {
		return OK(map[string]string{"value": "present-for-get"})
	}).WithOperationID("headConformance").Produces(http.StatusOK)

	server := httptest.NewServer(app.MustBuild())
	defer server.Close()

	req, err := http.NewRequest(http.MethodHead, server.URL+"/head", nil)
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
}
