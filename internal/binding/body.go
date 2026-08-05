package binding

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"reflect"
)

func decodeBody(r *http.Request, target reflect.Value, limit int64, disallowUnknown bool, required bool) error {
	if r.Body == nil {
		if required {
			return fmt.Errorf("request body is required")
		}
		return nil
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return fmt.Errorf("Content-Type must be application/json")
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if err != nil {
		return fmt.Errorf("read JSON body: %w", err)
	}
	if int64(len(data)) > limit {
		return fmt.Errorf("JSON body exceeds %d bytes", limit)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		if required {
			return fmt.Errorf("request body is required")
		}
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if disallowUnknown {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(target.Addr().Interface()); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("JSON body must contain exactly one value")
		}
		return fmt.Errorf("invalid trailing JSON: %w", err)
	}
	return nil
}
