package docs

import (
	"crypto/sha256"
	"fmt"
	"net/http"
)

type immutableJSONHandler struct {
	data []byte
	etag string
}

func NewOpenAPIHandler(data []byte) http.Handler {
	copyData := append([]byte(nil), data...)
	sum := sha256.Sum256(copyData)
	return immutableJSONHandler{data: copyData, etag: fmt.Sprintf(`"%x"`, sum)}
}
func (h immutableJSONHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("ETag", h.etag)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Header.Get("If-None-Match") == h.etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(h.data)
	}
}
