package oashttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPContextConformanceTypedHandlerObservesClientCancellation(t *testing.T) {
	entered := make(chan struct{})
	canceled := make(chan error, 1)

	app := New(Config{Info: Info{Title: "Context conformance", Version: "test"}})
	MapGet(app.Group(""), "/typed-cancel", func(ctx context.Context, _ struct{}) Result[struct{}] {
		close(entered)
		<-ctx.Done()
		canceled <- ctx.Err()
		return NoContent[struct{}]()
	}).WithOperationID("typedCancel").Produces(http.StatusNoContent)

	server := httptest.NewServer(app.MustBuild())
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/typed-cancel", nil)
	if err != nil {
		t.Fatal(err)
	}

	requestDone := make(chan error, 1)
	go func() {
		resp, err := server.Client().Do(req)
		if resp != nil {
			_ = resp.Body.Close()
		}
		requestDone <- err
	}()

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("typed handler was not entered")
	}
	cancel()

	select {
	case err := <-canceled:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("handler context error=%v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("typed handler did not observe request cancellation")
	}

	select {
	case <-requestDone:
	case <-time.After(2 * time.Second):
		t.Fatal("client request did not finish after cancellation")
	}
}

func TestHTTPContextConformanceRawHandlerObservesClientCancellation(t *testing.T) {
	entered := make(chan struct{})
	canceled := make(chan error, 1)

	app := New(Config{Info: Info{Title: "Raw context conformance", Version: "test"}})
	MapHandler(app.Group(""), http.MethodGet, "/raw-cancel", http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
		canceled <- r.Context().Err()
	})).WithOperationID("rawCancel").Produces(http.StatusNoContent)

	server := httptest.NewServer(app.MustBuild())
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/raw-cancel", nil)
	if err != nil {
		t.Fatal(err)
	}

	requestDone := make(chan error, 1)
	go func() {
		resp, err := server.Client().Do(req)
		if resp != nil {
			_ = resp.Body.Close()
		}
		requestDone <- err
	}()

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("raw handler was not entered")
	}
	cancel()

	select {
	case err := <-canceled:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("handler context error=%v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("raw handler did not observe request cancellation")
	}

	select {
	case <-requestDone:
	case <-time.After(2 * time.Second):
		t.Fatal("client request did not finish after cancellation")
	}
}
