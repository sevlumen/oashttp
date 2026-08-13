package oashttp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"net/textproto"
	"strings"
	"testing"
)

func TestRootRouteDoesNotCaptureUnknownPath(t *testing.T) {
	app := New(Config{Info: Info{Title: "Route conformance", Version: "1.0.0"}})
	MapGet(app.Group(""), "/", func(context.Context, struct{}) Result[struct{}] {
		return NoContent[struct{}]()
	}).WithOperationID("rootExact").Produces(http.StatusNoContent)

	handler := app.MustBuild()
	if status := serveStatus(handler, http.MethodGet, "/"); status != http.StatusNoContent {
		t.Fatalf("GET / status=%d", status)
	}
	if status := serveStatus(handler, http.MethodGet, "/unknown"); status != http.StatusNotFound {
		t.Fatalf("GET /unknown status=%d want=%d", status, http.StatusNotFound)
	}
}

func TestTrailingSlashRoutesDoNotCaptureDescendants(t *testing.T) {
	t.Run("typed", func(t *testing.T) {
		app := New(Config{Info: Info{Title: "Typed trailing slash", Version: "1.0.0"}})
		MapGet(app.Group(""), "/typed/", func(context.Context, struct{}) Result[struct{}] {
			return NoContent[struct{}]()
		}).WithOperationID("typedTrailingExact").Produces(http.StatusNoContent)

		handler := app.MustBuild()
		if status := serveStatus(handler, http.MethodGet, "/typed/"); status != http.StatusNoContent {
			t.Fatalf("GET /typed/ status=%d", status)
		}
		if status := serveStatus(handler, http.MethodGet, "/typed/child"); status != http.StatusNotFound {
			t.Fatalf("GET /typed/child status=%d want=%d", status, http.StatusNotFound)
		}
	})

	t.Run("raw", func(t *testing.T) {
		app := New(Config{Info: Info{Title: "Raw trailing slash", Version: "1.0.0"}})
		MapHandler(app.Group(""), http.MethodGet, "/raw/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})).WithOperationID("rawTrailingExact").Produces(http.StatusNoContent)

		handler := app.MustBuild()
		if status := serveStatus(handler, http.MethodGet, "/raw/"); status != http.StatusNoContent {
			t.Fatalf("GET /raw/ status=%d", status)
		}
		if status := serveStatus(handler, http.MethodGet, "/raw/child"); status != http.StatusNotFound {
			t.Fatalf("GET /raw/child status=%d want=%d", status, http.StatusNotFound)
		}
	})
}

func TestRecoveryPreservesInformationalThenFinalResponse(t *testing.T) {
	app := New(Config{
		Info:         Info{Title: "Informational response", Version: "1.0.0"},
		ErrorHandler: func(context.Context, error) {},
	})
	MapHandler(app.Group(""), http.MethodGet, "/early", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusEarlyHints)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	})).WithOperationID("earlyThenCreated").Produces(http.StatusCreated)

	resp, informational := requestWith1xxTrace(t, app.MustBuild(), "/early")
	if len(informational) != 1 || informational[0] != http.StatusEarlyHints {
		t.Fatalf("informational=%v", informational)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("final status=%d want=%d", resp.StatusCode, http.StatusCreated)
	}
}

func TestRecoveryWrites500AfterInformationalPanic(t *testing.T) {
	app := New(Config{
		Info:         Info{Title: "Informational panic", Version: "1.0.0"},
		ErrorHandler: func(context.Context, error) {},
	})
	MapHandler(app.Group(""), http.MethodGet, "/early-panic", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusEarlyHints)
		panic("boom")
	})).WithOperationID("earlyThenPanic").ProducesProblem(http.StatusInternalServerError)

	resp, informational := requestWith1xxTrace(t, app.MustBuild(), "/early-panic")
	if len(informational) != 1 || informational[0] != http.StatusEarlyHints {
		t.Fatalf("informational=%v", informational)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("final status=%d want=%d", resp.StatusCode, http.StatusInternalServerError)
	}
}

func TestInformationalResultStatusReturns500(t *testing.T) {
	for _, status := range []int{http.StatusContinue, http.StatusSwitchingProtocols, http.StatusEarlyHints, 199} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var reported error
			recorder := httptest.NewRecorder()
			JSON(status, struct{}{}).WriteHTTP(recorder, func(err error) { reported = err })
			if recorder.Code != http.StatusInternalServerError {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			want := fmt.Sprintf("invalid result status %d", status)
			if reported == nil || !strings.Contains(reported.Error(), want) {
				t.Fatalf("reported=%v want substring=%q", reported, want)
			}
		})
	}
}

func serveStatus(handler http.Handler, method, path string) int {
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(method, path, nil))
	return recorder.Code
}

func requestWith1xxTrace(t *testing.T, handler http.Handler, path string) (*http.Response, []int) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	var informational []int
	req, err := http.NewRequest(http.MethodGet, server.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	trace := &httptrace.ClientTrace{
		Got1xxResponse: func(code int, _ textproto.MIMEHeader) error {
			informational = append(informational, code)
			return nil
		},
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp, informational
}
