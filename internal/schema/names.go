package schema

import (
	"reflect"
	"strconv"
	"strings"
)

func (r *Registry) componentName(t reflect.Type) string {
	if name, ok := r.names[t]; ok {
		return name
	}
	base := t.Name()
	if base == "" {
		base = "Anonymous"
	}
	name := base
	if other, ok := r.used[name]; ok && other != t {
		pkg := strings.NewReplacer("/", "_", ".", "_", "-", "_").Replace(t.PkgPath())
		name = pkg + "_" + base
		for suffix := 2; ; suffix++ {
			if existing, exists := r.used[name]; !exists || existing == t {
				break
			}
			name = pkg + "_" + base + "_" + strconv.Itoa(suffix)
		}
	}
	r.names[t] = name
	r.used[name] = t
	return name
}
