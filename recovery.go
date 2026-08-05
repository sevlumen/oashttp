package oashttp

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/quang020102/go-osm/internal/core"
	internalfailure "github.com/quang020102/go-osm/internal/failure"
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
	w.wroteHeader = true
	w.status = status
	w.ResponseWriter.WriteHeader(status)
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
		next.ServeHTTP(state, r)
	})
}

func reportRuntimeError(ctx context.Context, handler func(context.Context, error), err error) {
	if handler == nil {
		return
	}
	defer func() { _ = recover() }()
	handler(ctx, err)
}
