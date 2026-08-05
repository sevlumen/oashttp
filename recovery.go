package oashttp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/quang020102/go-osm/internal/core"
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

func recoverPanics(next http.Handler, onError func(context.Context, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state := &responseState{ResponseWriter: w}
		defer func() {
			if recovered := recover(); recovered != nil {
				err := fmt.Errorf("oashttp: recovered panic: %v\n%s", recovered, debug.Stack())
				reportRuntimeError(r.Context(), onError, err)
				if !state.wroteHeader {
					writeRecoveryProblem(state)
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

func writeRecoveryProblem(w http.ResponseWriter) {
	payload, _ := json.Marshal(core.ProblemDetails{
		Title:  http.StatusText(http.StatusInternalServerError),
		Status: http.StatusInternalServerError,
		Code:   "INTERNAL_SERVER_ERROR",
		Detail: "An unexpected error occurred",
	})
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write(payload)
}
