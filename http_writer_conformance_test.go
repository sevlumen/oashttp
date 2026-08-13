package oashttp

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type plainCapabilityWriter struct {
	header http.Header
	status int
}

func (w *plainCapabilityWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}
func (w *plainCapabilityWriter) WriteHeader(status int) { w.status = status }
func (w *plainCapabilityWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return len(data), nil
}

type flusherCapabilityWriter struct {
	plainCapabilityWriter
	flushed bool
}

func (w *flusherCapabilityWriter) Flush() { w.flushed = true }

type hijackerCapabilityWriter struct {
	plainCapabilityWriter
	err error
}

func (w *hijackerCapabilityWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, w.err
}

type deadlineCapabilityWriter struct {
	plainCapabilityWriter
	deadline time.Time
}

func (w *deadlineCapabilityWriter) SetWriteDeadline(deadline time.Time) error {
	w.deadline = deadline
	return nil
}

func TestHTTPWriterConformanceDoesNotAdvertiseMissingCapabilities(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, ok := w.(http.Flusher); ok {
			t.Fatal("recovery wrapper falsely advertised http.Flusher")
		}
		if _, ok := w.(http.Hijacker); ok {
			t.Fatal("recovery wrapper falsely advertised http.Hijacker")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	handler := recoverPanics(next, nil, ProblemDetailsFormatter{}, "application/problem+json")
	writer := &plainCapabilityWriter{}
	handler.ServeHTTP(writer, httptest.NewRequest(http.MethodGet, "/", nil))
	if writer.status != http.StatusNoContent {
		t.Fatalf("status=%d", writer.status)
	}
}

func TestHTTPWriterConformancePreservesFlusher(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, ok := w.(http.Flusher); !ok {
			t.Fatal("recovery wrapper hid http.Flusher")
		}
		w.(http.Flusher).Flush()
	})
	handler := recoverPanics(next, nil, ProblemDetailsFormatter{}, "application/problem+json")
	writer := &flusherCapabilityWriter{}
	handler.ServeHTTP(writer, httptest.NewRequest(http.MethodGet, "/", nil))
	if !writer.flushed {
		t.Fatal("underlying Flush was not called")
	}
}

func TestHTTPWriterConformancePreservesHijacker(t *testing.T) {
	sentinel := errors.New("hijack sentinel")
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("recovery wrapper hid http.Hijacker")
		}
		_, _, err := hijacker.Hijack()
		if !errors.Is(err, sentinel) {
			t.Fatalf("Hijack error=%v", err)
		}
	})
	handler := recoverPanics(next, nil, ProblemDetailsFormatter{}, "application/problem+json")
	writer := &hijackerCapabilityWriter{err: sentinel}
	handler.ServeHTTP(writer, httptest.NewRequest(http.MethodGet, "/", nil))
}

func TestHTTPWriterConformanceResponseControllerUsesUnwrap(t *testing.T) {
	deadline := time.Now().Add(time.Minute).Round(0)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := http.NewResponseController(w).SetWriteDeadline(deadline); err != nil {
			t.Fatalf("SetWriteDeadline: %v", err)
		}
	})
	handler := recoverPanics(next, nil, ProblemDetailsFormatter{}, "application/problem+json")
	writer := &deadlineCapabilityWriter{}
	handler.ServeHTTP(writer, httptest.NewRequest(http.MethodGet, "/", nil))
	if !writer.deadline.Equal(deadline) {
		t.Fatalf("deadline=%v want=%v", writer.deadline, deadline)
	}
}

func TestHTTPWriterConformanceRawFlushStreamsBeforeHandlerReturns(t *testing.T) {
	release := make(chan struct{})
	app := New(Config{Info: Info{Title: "Streaming conformance", Version: "test"}})
	MapHandler(app.Group(""), http.MethodGet, "/stream", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			panic("http.Flusher unavailable")
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("first\n"))
		flusher.Flush()
		<-release
		_, _ = w.Write([]byte("second\n"))
	})).WithOperationID("stream").Produces(http.StatusOK)

	server := httptest.NewServer(app.MustBuild())
	defer server.Close()

	type responseResult struct {
		resp *http.Response
		err  error
	}
	responseCh := make(chan responseResult, 1)
	go func() {
		resp, err := server.Client().Get(server.URL + "/stream")
		responseCh <- responseResult{resp: resp, err: err}
	}()

	var resp *http.Response
	select {
	case result := <-responseCh:
		if result.err != nil {
			close(release)
			t.Fatal(result.err)
		}
		resp = result.resp
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("client did not receive flushed response headers")
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	first, err := reader.ReadString('\n')
	if err != nil {
		close(release)
		t.Fatal(err)
	}
	if first != "first\n" {
		close(release)
		t.Fatalf("first chunk=%q", first)
	}

	close(release)
	second, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if second != "second\n" {
		t.Fatalf("second chunk=%q", second)
	}
}

func TestHTTPWriterConformancePanicAfterCommitDoesNotWriteSecondResponse(t *testing.T) {
	reported := make(chan error, 1)
	app := New(Config{
		Info: Info{Title: "Committed panic", Version: "test"},
		ErrorHandler: func(_ context.Context, err error) {
			reported <- err
		},
	})
	MapHandler(app.Group(""), http.MethodGet, "/committed-panic", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("accepted"))
		panic("after commit")
	})).WithOperationID("committedPanic").Produces(http.StatusAccepted)

	recorder := httptest.NewRecorder()
	app.MustBuild().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/committed-panic", nil))

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if recorder.Body.String() != "accepted" {
		t.Fatalf("body=%q", recorder.Body.String())
	}
	select {
	case err := <-reported:
		if err == nil {
			t.Fatal("reported nil panic error")
		}
	default:
		t.Fatal("panic was not reported")
	}
}
