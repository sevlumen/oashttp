package schema

import (
	"reflect"
	"sort"
	"strings"
	"unicode"
)

type jsonTag struct {
	Name      string
	HasName   bool
	OmitEmpty bool
	String    bool
	Skip      bool
}

type jsonField struct {
	Field  reflect.StructField
	Index  []int
	Name   string
	Tagged bool
	Quoted bool
}

func parseJSONTag(field reflect.StructField) jsonTag {
	raw := field.Tag.Get("json")
	if raw == "-" {
		return jsonTag{Skip: true}
	}

	name, options, hasOptions := strings.Cut(raw, ",")
	out := jsonTag{}
	if validJSONTagName(name) {
		out.Name = name
		out.HasName = true
	}
	if !hasOptions {
		return out
	}
	for _, option := range strings.Split(options, ",") {
		switch option {
		case "omitempty":
			out.OmitEmpty = true
		case "string":
			out.String = true
		}
	}
	return out
}

func validJSONTagName(name string) bool {
	if name == "" {
		return false
	}
	const punctuation = "!#$%&()*+-./:;<=>?@[]^_{|}~ "
	for _, r := range name {
		if strings.ContainsRune(punctuation, r) || unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		return false
	}
	return true
}

func quotedJSONType(t reflect.Type) bool {
	if t.Name() == "" && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Bool,
		reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

func resolveJSONFields(t reflect.Type) []jsonField {
	if t.Kind() != reflect.Struct {
		return nil
	}
	candidates := make([]jsonField, 0, t.NumField())
	collectJSONFields(t, nil, map[reflect.Type]bool{}, &candidates)

	byName := make(map[string][]jsonField, len(candidates))
	for _, candidate := range candidates {
		byName[candidate.Name] = append(byName[candidate.Name], candidate)
	}

	selected := make([]jsonField, 0, len(byName))
	for _, group := range byName {
		minDepth := len(group[0].Index)
		for _, candidate := range group[1:] {
			if len(candidate.Index) < minDepth {
				minDepth = len(candidate.Index)
			}
		}

		shallow := make([]jsonField, 0, len(group))
		hasTagged := false
		for _, candidate := range group {
			if len(candidate.Index) != minDepth {
				continue
			}
			shallow = append(shallow, candidate)
			if candidate.Tagged {
				hasTagged = true
			}
		}
		if hasTagged {
			tagged := shallow[:0]
			for _, candidate := range shallow {
				if candidate.Tagged {
					tagged = append(tagged, candidate)
				}
			}
			shallow = tagged
		}
		if len(shallow) == 1 {
			selected = append(selected, shallow[0])
		}
	}

	sort.Slice(selected, func(i, j int) bool {
		return compareJSONIndex(selected[i].Index, selected[j].Index) < 0
	})
	return selected
}

func collectJSONFields(t reflect.Type, prefix []int, ancestors map[reflect.Type]bool, out *[]jsonField) {
	if ancestors[t] {
		return
	}
	nextAncestors := make(map[reflect.Type]bool, len(ancestors)+1)
	for typ, present := range ancestors {
		nextAncestors[typ] = present
	}
	nextAncestors[t] = true

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := parseJSONTag(field)
		if tag.Skip {
			continue
		}

		embeddedType := field.Type
		if field.Anonymous && embeddedType.Kind() == reflect.Pointer {
			embeddedType = embeddedType.Elem()
		}
		if field.Anonymous {
			if field.PkgPath != "" && embeddedType.Kind() != reflect.Struct {
				continue
			}
		} else if field.PkgPath != "" {
			continue
		}

		index := append(append([]int(nil), prefix...), i)
		if field.Anonymous && !tag.HasName && embeddedType.Kind() == reflect.Struct {
			collectJSONFields(embeddedType, index, nextAncestors, out)
			continue
		}

		name := field.Name
		if tag.HasName {
			name = tag.Name
		}
		*out = append(*out, jsonField{
			Field:  field,
			Index:  index,
			Name:   name,
			Tagged: tag.HasName,
			Quoted: tag.String && quotedJSONType(field.Type),
		})
	}
}

func compareJSONIndex(left, right []int) int {
	limit := len(left)
	if len(right) < limit {
		limit = len(right)
	}
	for i := 0; i < limit; i++ {
		if left[i] < right[i] {
			return -1
		}
		if left[i] > right[i] {
			return 1
		}
	}
	if len(left) < len(right) {
		return -1
	}
	if len(left) > len(right) {
		return 1
	}
	return 0
}
