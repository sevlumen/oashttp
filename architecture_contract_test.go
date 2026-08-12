package oashttp

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const architectureModulePath = "github.com/sevlumen/oashttp/v2"

func importsInDir(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}

	var imports []string
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		path := filepath.Join(dir, name)
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse imports from %s: %v", path, err)
		}
		for _, spec := range file.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatalf("unquote import %s in %s: %v", spec.Path.Value, path, err)
			}
			imports = append(imports, importPath)
		}
	}
	return imports
}

func TestArchitectureContracts(t *testing.T) {
	for _, dir := range []string{"internal/core", "internal/oas31", "internal/httpsem", "internal/validationrule"} {
		t.Run(strings.TrimPrefix(dir, "internal/")+" remains a dependency leaf", func(t *testing.T) {
			for _, importPath := range importsInDir(t, dir) {
				if importPath == architectureModulePath || strings.HasPrefix(importPath, architectureModulePath+"/internal/") {
					t.Fatalf("%s imports %q; dependency-leaf packages must not depend on the public facade or sibling internal packages", dir, importPath)
				}
			}
		})
	}

	entries, err := os.ReadDir("internal")
	if err != nil {
		t.Fatalf("read internal packages: %v", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join("internal", entry.Name())
		t.Run(entry.Name()+" does not import public facade", func(t *testing.T) {
			for _, importPath := range importsInDir(t, dir) {
				if importPath == architectureModulePath {
					t.Fatalf("%s imports public facade %q", dir, importPath)
				}
			}
		})
	}
}
