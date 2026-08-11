package oashttp

import (
	"net/http"

	"github.com/sevlumen/oashttp/v2/internal/core"
)

func operationCarrierMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := core.WithOperationCarrier(r.Context())
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
