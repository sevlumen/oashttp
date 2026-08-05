package binding

import (
	"github.com/quang020102/go-osm/internal/route"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

type Body struct {
	FullName string `json:"fullName"`
}
type UpdateInput struct {
	ID      string `path:"id"`
	DryRun  bool   `query:"dryRun"`
	TraceID string `header:"X-Trace-ID"`
	Body    Body   `body:"json"`
}

func TestBindPathQueryHeaderAndJSON(t *testing.T) {
	pattern, _ := route.Parse("/users/{id:uuid}")
	p, e := Compile(reflect.TypeOf(UpdateInput{}), pattern, Options{JSONBodyLimit: 1 << 20, DisallowUnknownJSONFields: true})
	if e != nil {
		t.Fatal(e)
	}
	r := httptest.NewRequest(http.MethodPut, "/users/550e8400-e29b-41d4-a716-446655440000?dryRun=true", strings.NewReader(`{"fullName":"Alice"}`))
	r.SetPathValue("id", "550e8400-e29b-41d4-a716-446655440000")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Trace-ID", "trace-1")
	v, errs := p.Bind(r)
	if len(errs) != 0 {
		t.Fatalf("errors=%#v", errs)
	}
	input := v.Interface().(UpdateInput)
	if !input.DryRun || input.TraceID != "trace-1" {
		t.Fatalf("input=%#v", input)
	}
}

type dateInput struct {
	Date time.Time `path:"date"`
}
type optionalBodyInput struct {
	Body *Body `body:"json"`
}
type queryInput struct {
	Page int `query:"page"`
}
type textValue string

func (v *textValue) UnmarshalText(data []byte) error {
	*v = textValue(strings.ToUpper(string(data)))
	return nil
}

type textInput struct {
	Value textValue `query:"value"`
}

func TestBindDateConstraintIntoTime(t *testing.T) {
	pattern, _ := route.Parse("/reports/{date:date}")
	plan, err := Compile(reflect.TypeOf(dateInput{}), pattern, Options{})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/reports/2026-08-05", nil)
	request.SetPathValue("date", "2026-08-05")
	value, fieldErrors := plan.Bind(request)
	if len(fieldErrors) != 0 {
		t.Fatalf("errors=%#v", fieldErrors)
	}
	if got := value.Interface().(dateInput).Date.Format("2006-01-02"); got != "2026-08-05" {
		t.Fatalf("date=%q", got)
	}
}
func TestOptionalPointerBodyPreservesJSONNull(t *testing.T) {
	pattern, _ := route.Parse("/optional")
	plan, err := Compile(reflect.TypeOf(optionalBodyInput{}), pattern, Options{})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/optional", strings.NewReader("null"))
	request.Header.Set("Content-Type", "application/json")
	value, fieldErrors := plan.Bind(request)
	if len(fieldErrors) != 0 {
		t.Fatalf("errors=%#v", fieldErrors)
	}
	if value.Interface().(optionalBodyInput).Body != nil {
		t.Fatal("JSON null must remain nil")
	}
}
func TestBindRejectsUnknownTrailingOversizedAndDuplicateValues(t *testing.T) {
	pattern, _ := route.Parse("/users/{id:string}")
	plan, err := Compile(reflect.TypeOf(UpdateInput{}), pattern, Options{JSONBodyLimit: 32, DisallowUnknownJSONFields: true})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, body, url string }{
		{"unknown", `{"fullName":"Alice","extra":true}`, "/users/u-1"},
		{"trailing", `{"fullName":"Alice"} {}`, "/users/u-1"},
		{"oversized", `{"fullName":"abcdefghijklmnopqrstuvwxyz"}`, "/users/u-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPut, tc.url, strings.NewReader(tc.body))
			request.SetPathValue("id", "u-1")
			request.Header.Set("Content-Type", "application/json")
			_, errs := plan.Bind(request)
			if len(errs) == 0 {
				t.Fatal("expected binding error")
			}
		})
	}
	queryPlan, err := Compile(reflect.TypeOf(queryInput{}), route.Pattern{UserPath: "/query", ServeMuxPath: "/query", OpenAPIPath: "/query"}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/query?page=1&page=2", nil)
	_, errs := queryPlan.Bind(request)
	if len(errs) != 1 {
		t.Fatalf("errors=%#v", errs)
	}
}
func TestBindUsesTextUnmarshaler(t *testing.T) {
	pattern, _ := route.Parse("/text")
	plan, err := Compile(reflect.TypeOf(textInput{}), pattern, Options{})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/text?value=hello", nil)
	value, errs := plan.Bind(request)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	if got := value.Interface().(textInput).Value; got != "HELLO" {
		t.Fatalf("value=%q", got)
	}
}
