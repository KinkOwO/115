package gamedata

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

type Difference struct {
	Path string `json:"path"`
	JSON string `json:"json"`
	PVF  string `json:"pvf"`
}

type Comparison struct {
	Count       int          `json:"difference_count"`
	Differences []Difference `json:"differences"`
	Truncated   bool         `json:"truncated"`
}

// Compare walks typed catalogs without serializing entire source token trees.
// Only top-level archive metadata is excluded; callers must compare checksums
// separately. Missing keys and unequal array lengths each count as a difference.
// Nil and empty collections are equivalent; collection order remains significant.
func Compare(legacy, direct any, limit int) Comparison {
	if limit < 0 {
		limit = 0
	}
	r := Comparison{Differences: []Difference{}}
	add := func(path string, a, b reflect.Value) {
		r.Count++
		if len(r.Differences) < limit {
			r.Differences = append(r.Differences, Difference{path, describe(a), describe(b)})
		}
	}
	var walk func(string, reflect.Value, reflect.Value)
	walk = func(path string, a, b reflect.Value) {
		if !a.IsValid() || !b.IsValid() {
			add(path, a, b)
			return
		}
		if a.Type() != b.Type() {
			add(path, a, b)
			return
		}
		switch a.Kind() {
		case reflect.Pointer, reflect.Interface:
			if a.IsNil() && b.IsNil() {
				return
			}
			if a.IsNil() || b.IsNil() {
				add(path, a, b)
				return
			}
			walk(path, a.Elem(), b.Elem())
		case reflect.Struct:
			for i := 0; i < a.NumField(); i++ {
				field := a.Type().Field(i)
				if field.PkgPath != "" {
					continue
				}
				name := strings.Split(field.Tag.Get("json"), ",")[0]
				if path == "" && name == "source" {
					continue
				}
				// Runtime-only exported indexes (json:"-") affect gameplay too.
				if name == "" || name == "-" {
					name = field.Name
				}
				walk(path+"/"+escape(name), a.Field(i), b.Field(i))
			}
		case reflect.Map:
			keys := map[string]reflect.Value{}
			for _, value := range []reflect.Value{a, b} {
				for _, k := range value.MapKeys() {
					keys[fmt.Sprint(k.Interface())] = k
				}
			}
			order := make([]string, 0, len(keys))
			for k := range keys {
				order = append(order, k)
			}
			sort.Strings(order)
			for _, k := range order {
				walk(path+"/"+escape(k), a.MapIndex(keys[k]), b.MapIndex(keys[k]))
			}
		case reflect.Slice, reflect.Array:
			if a.Len() != b.Len() {
				add(path+"/length", reflect.ValueOf(a.Len()), reflect.ValueOf(b.Len()))
			}
			for i := 0; i < min(a.Len(), b.Len()); i++ {
				walk(path+"/"+strconv.Itoa(i), a.Index(i), b.Index(i))
			}
		default:
			if !reflect.DeepEqual(a.Interface(), b.Interface()) {
				add(path, a, b)
			}
		}
	}
	walk("", reflect.ValueOf(legacy), reflect.ValueOf(direct))
	r.Truncated = r.Count > len(r.Differences)
	return r
}

func escape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}

func describe(v reflect.Value) string {
	if !v.IsValid() {
		return "<missing>"
	}
	switch v.Kind() {
	case reflect.Map, reflect.Slice, reflect.Array:
		return fmt.Sprintf("%s length=%d", v.Type(), v.Len())
	case reflect.Struct:
		return v.Type().String()
	}
	b, err := json.Marshal(v.Interface())
	if err != nil {
		return fmt.Sprint(v.Interface())
	}
	runes := []rune(string(b))
	if len(runes) > 256 {
		return string(runes[:256]) + "…"
	}
	return string(b)
}
