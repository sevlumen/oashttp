package oashttp

import (
	"fmt"
	"strings"
)

type Group struct {
	app    *App
	prefix string
}

func (a *App) Group(prefix string) *Group {
	normalized, err := normalizePrefix(prefix)
	if err != nil {
		panic(err)
	}
	return &Group{app: a, prefix: normalized}
}
func (g *Group) Group(prefix string) *Group {
	normalized, err := normalizePrefix(prefix)
	if err != nil {
		panic(err)
	}
	return &Group{app: g.app, prefix: joinPaths(g.prefix, normalized)}
}
func normalizePrefix(prefix string) (string, error) {
	if prefix == "" || prefix == "/" {
		return "", nil
	}
	if !strings.HasPrefix(prefix, "/") {
		return "", fmt.Errorf("group prefix %q must start with /", prefix)
	}
	if strings.ContainsAny(prefix, "{}?#\x00") {
		return "", fmt.Errorf("group prefix %q cannot contain route parameters", prefix)
	}
	if strings.Contains(prefix, "//") {
		return "", fmt.Errorf("group prefix %q contains an empty segment", prefix)
	}
	return strings.TrimSuffix(prefix, "/"), nil
}
func joinPaths(prefix, path string) string {
	if prefix == "" {
		if path == "" {
			return "/"
		}
		return path
	}
	if path == "" || path == "/" {
		return prefix
	}
	return prefix + "/" + strings.TrimPrefix(path, "/")
}
