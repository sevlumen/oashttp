package oashttp

import "testing"

func TestGroupComposition(t *testing.T) {
	a := New(Config{})
	g := a.Group("/api/v1").Group("/core")
	if g.prefix != "/api/v1/core" {
		t.Fatalf("prefix=%q", g.prefix)
	}
}
