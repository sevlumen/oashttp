package route

import "testing"

func TestParseCompilesExactTrailingSlashServeMuxPatterns(t *testing.T) {
	tests := []struct {
		path         string
		serveMuxPath string
	}{
		{path: "/", serveMuxPath: "/{$}"},
		{path: "/users/", serveMuxPath: "/users/{$}"},
		{path: "/users/{id}/", serveMuxPath: "/users/{id}/{$}"},
		{path: "/users/{id}", serveMuxPath: "/users/{id}"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got, err := Parse(tt.path)
			if err != nil {
				t.Fatal(err)
			}
			if got.ServeMuxPath != tt.serveMuxPath {
				t.Fatalf("ServeMuxPath=%q want=%q", got.ServeMuxPath, tt.serveMuxPath)
			}
			if got.OpenAPIPath != tt.path {
				t.Fatalf("OpenAPIPath=%q want=%q", got.OpenAPIPath, tt.path)
			}
		})
	}
}
