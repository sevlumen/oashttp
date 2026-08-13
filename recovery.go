package oashttp

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"

	"github.com/sevlumen/oashttp/v2/internal/core"
	internalfailure "github.com/sevlumen/oashttp/v2/internal/failure"
)

type responseState struct {
	http.ResponseWriter
	wroteHeader bool
	status      int
}

func (w *responseState) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	if status >= 100 && status < 200 && status != http.StatusSwitchingProtocols {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	w.ResponseWriter.WriteHeader(status)
	w.wroteHeader = true
	w.status = status
}

func (w *responseState) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

// Unwrap allows http.ResponseController to reach optional capabilities exposed
// by the underlying response writer without falsely advertising them here.
func (w *responseState) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *responseState) flush() {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	w.ResponseWriter.(http.Flusher).Flush()
}

func (w *responseState) hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, rw, err := w.ResponseWriter.(http.Hijacker).Hijack()
	if err == nil {
		// Once the connection is taken over, recovery must not attempt to write
		// an HTTP failure response through the original ResponseWriter.
		w.wroteHeader = true
	}
	return conn, rw, err
}

type responseStateFlusher struct{ *responseState }

func (w *responseStateFlusher) Flush() { w.responseState.flush() }

type responseStateHijacker struct{ *responseState }

func (w *responseStateHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.responseState.hijack()
}

type responseStateFlusherHijacker struct{ *responseState }

func (w *responseStateFlusherHijacker) Flush() { w.responseState.flush() }
func (w *responseStateFlusherHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.responseState.hijack()
}

func (w *responseState) capabilityWriter() http.ResponseWriter {
	_, hasFlusher := w.ResponseWriter.(http.Flusher)
	_, hasHijacker := w.ResponseWriter.(http.Hijacker)
	switch {
	case hasFlusher && hasHijacker:
		return &responseStateFlusherHijacker{responseState: w}
	case hasFlusher:
		return &responseStateFlusher{responseState: w}
	case hasHijacker:
		return &responseStateHijacker{responseState: w}
	default:
		return w
	}
}

func recoverPanics(next http.Handler, onError func(context.Context, error), formatter core.FailureFormatter, failureContentType string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state := &responseState{ResponseWriter: w}
		defer func() {
			if recovered := recover(); recovered != nil {
				err := fmt.Errorf("oashttp: recovered panic: %v\n%s", recovered, debug.Stack())
				reportRuntimeError(r.Context(), onError, err)
				if !state.wroteHeader {
					internalfailure.WriteResolved(state, formatter, failureContentType, core.Failure{
						Status: http.StatusInternalServerError,
						Code:   "INTERNAL_SERVER_ERROR",
						Detail: "An unexpected error occurred",
					}, func(writeErr error) { reportRuntimeError(r.Context(), onError, writeErr) })
				}
			}
		}()
		next.ServeHTTP(state.capabilityWriter(), r)
	})
}

func reportRuntimeError(ctx context.Context, handler func(context.Context, error), err error) {
	if handler == nil {
		return
	}
	defer func() { _ = recover() }()
	handler(ctx, err)
}
