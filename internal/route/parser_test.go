package route

import (
	"reflect"
	"testing"
)

func TestParseConstrainedRoute(t *testing.T) {
	got, err := Parse("/users/{id:uuid}")
	if err != nil {
		t.Fatal(err)
	}
	if got.ServeMuxPath != "/users/{id}" || got.OpenAPIPath != "/users/{id}" {
		t.Fatalf("pattern=%#v", got)
	}
	want := []Parameter{{Name: "id", Constraint: "uuid"}}
	if !reflect.DeepEqual(got.Parameters, want) {
		t.Fatalf("parameters=%#v", got.Parameters)
	}
}

func TestParseRejectsUnsupportedWildcardAndInvalidNames(t *testing.T) {
	for _, value := range []string{"/files/{path...}", "/users/{1id}", "/users/{id/name}", "/users/{id}?x=1"} {
		if _, err := Parse(value); err == nil {
			t.Fatalf("Parse(%q) succeeded", value)
		}
	}
}
