package oashttp

import (
	"fmt"
	"strings"
)

type Group struct {
	app         *App
	prefix      string
	middlewares []Middleware
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
	return &Group{
		app:         g.app,
		prefix:      joinPaths(g.prefix, normalized),
		middlewares: append([]Middleware(nil), g.middlewares...),
	}
}

// Use appends middleware to this group. It applies to operations registered
// after the call and is inherited by child groups created afterwards.
func (g *Group) Use(middleware Middleware) error {
	if middleware == nil {
		return fmt.Errorf("middleware cannot be nil")
	}
	if g == nil || g.app == nil {
		return fmt.Errorf("group is nil")
	}
	g.app.mu.Lock()
	defer g.app.mu.Unlock()
	if g.app.frozen {
		return ErrFrozen
	}
	g.middlewares = append(g.middlewares, middleware)
	return nil
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
