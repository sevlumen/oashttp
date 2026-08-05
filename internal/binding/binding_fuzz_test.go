package binding

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/oashttp/oashttp/internal/route"
)

type fuzzInput struct {
	Value int            `query:"value"`
	Body  map[string]any `body:"json"`
}

func FuzzCompiledBinderNeverPanics(f *testing.F) {
	pattern, _ := route.Parse("/fuzz")
	plan, err := Compile(reflect.TypeOf(fuzzInput{}), pattern, Options{JSONBodyLimit: 256, DisallowUnknownJSONFields: true})
	if err != nil {
		f.Fatal(err)
	}
	f.Add("1", `{"ok":true}`)
	f.Add("bad", `{`)
	f.Add("", "")
	f.Fuzz(func(t *testing.T, query, body string) {
		req := httptest.NewRequest(http.MethodPost, "/fuzz?value="+url.QueryEscape(query), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		_, _ = plan.Bind(req)
	})
}
