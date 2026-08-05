package oashttp

import (
	"testing"

	"github.com/quang020102/go-osm/internal/route"
)

func FuzzRouteParserNeverPanics(f *testing.F) {
	for _, seed := range []string{"/users/{id:uuid}", "/health", "", "/{", "/users/{id:unknown}"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) { _, _ = route.Parse(value) })
}
