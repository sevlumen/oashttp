package route

import (
	"fmt"
	"strings"
)

type Parameter struct {
	Name       string
	Constraint string
}
type Pattern struct {
	UserPath     string
	ServeMuxPath string
	OpenAPIPath  string
	Parameters   []Parameter
}

func Parse(path string) (Pattern, error) {
	if path == "" || !strings.HasPrefix(path, "/") {
		return Pattern{}, fmt.Errorf("route %q must start with /", path)
	}
	if strings.ContainsAny(path, "?#\x00") {
		return Pattern{}, fmt.Errorf("route %q contains characters that are not valid in a path pattern", path)
	}
	if strings.Contains(path, "//") {
		return Pattern{}, fmt.Errorf("route %q contains an empty segment", path)
	}
	var out strings.Builder
	seen := map[string]struct{}{}
	parameters := []Parameter{}
	for i := 0; i < len(path); {
		if path[i] == '}' {
			return Pattern{}, fmt.Errorf("route %q has unmatched }", path)
		}
		if path[i] != '{' {
			out.WriteByte(path[i])
			i++
			continue
		}
		end := strings.IndexByte(path[i+1:], '}')
		if end < 0 {
			return Pattern{}, fmt.Errorf("route %q has unmatched {", path)
		}
		end += i + 1
		token := path[i+1 : end]
		if token == "" || strings.ContainsAny(token, "{}") {
			return Pattern{}, fmt.Errorf("route %q has an invalid parameter", path)
		}
		parts := strings.Split(token, ":")
		if len(parts) > 2 || parts[0] == "" {
			return Pattern{}, fmt.Errorf("route %q has an invalid parameter %q", path, token)
		}
		name, constraint := parts[0], "string"
		if !validParameterName(name) {
			return Pattern{}, fmt.Errorf("route %q has invalid parameter name %q", path, name)
		}
		if len(parts) == 2 {
			constraint = parts[1]
		}
		if _, ok := Constraints[constraint]; !ok {
			return Pattern{}, fmt.Errorf("route %q parameter %q uses unknown constraint %q", path, name, constraint)
		}
		if _, ok := seen[name]; ok {
			return Pattern{}, fmt.Errorf("route %q repeats parameter %q", path, name)
		}
		seen[name] = struct{}{}
		parameters = append(parameters, Parameter{Name: name, Constraint: constraint})
		out.WriteByte('{')
		out.WriteString(name)
		out.WriteByte('}')
		i = end + 1
	}
	normalized := out.String()
	return Pattern{UserPath: path, ServeMuxPath: normalized, OpenAPIPath: normalized, Parameters: parameters}, nil
}

func validParameterName(name string) bool {
	if name == "" {
		return false
	}
	for index, r := range name {
		if index == 0 {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_') {
				return false
			}
			continue
		}
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}
