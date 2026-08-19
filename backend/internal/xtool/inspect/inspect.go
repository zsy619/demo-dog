// Package inspect 提供对任意值的反射检视能力，
// 输出结构、深度、字段类型等元数据用于调试与文档生成。
package inspect

import (
	"fmt"
	"reflect"
	"strings"
)

// Field 是字段的描述。
type Field struct {
	Path  string `json:"path"`
	Type  string `json:"type"`
	Kind  string `json:"kind"`
	Tag   string `json:"tag,omitempty"`
	Depth int    `json:"depth"`
}

// Summary 是检视结果摘要。
type Summary struct {
	Type   string `json:"type"`
	Kind   string `json:"kind"`
	Depth  int    `json:"depth"`
	Fields int    `json:"fields"`
	Path   string `json:"path"`
}

// Of 返回一个值的概要信息。
func Of(v any) Summary {
	rt := reflect.TypeOf(v)
	return Summary{
		Type:   rt.String(),
		Kind:   rt.Kind().String(),
		Depth:  depthOf(rt, 0),
		Fields: countFields(rt),
		Path:   rt.PkgPath(),
	}
}

// Fields 递归枚举一个值的字段路径与类型。
func Fields(v any, maxDepth int) []Field {
	if maxDepth <= 0 {
		maxDepth = 8
	}
	var out []Field
	rt := reflect.TypeOf(v)
	walkFields(rt, "", 0, maxDepth, &out)
	return out
}

// String 返回一个简洁的多行描述。
func String(v any) string {
	s := Of(v)
	var b strings.Builder
	fmt.Fprintf(&b, "type=%s kind=%s depth=%d fields=%d", s.Type, s.Kind, s.Depth, s.Fields)
	return b.String()
}

func depthOf(rt reflect.Type, d int) int {
	if rt == nil {
		return d
	}
	k := rt.Kind()
	if k != reflect.Struct && k != reflect.Ptr && k != reflect.Slice && k != reflect.Array && k != reflect.Map {
		return d
	}
	if d > 16 {
		return d
	}
	switch k {
	case reflect.Ptr, reflect.Slice, reflect.Array, reflect.Map:
		elem := rt.Elem()
		if k == reflect.Map {
			elem = rt.Elem()
		}
		return depthOf(elem, d+1)
	case reflect.Struct:
		max := d
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			if !f.IsExported() {
				continue
			}
			cd := depthOf(f.Type, d+1)
			if cd > max {
				max = cd
			}
		}
		return max
	}
	return d
}

func countFields(rt reflect.Type) int {
	if rt == nil || rt.Kind() != reflect.Struct {
		return 0
	}
	n := 0
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if !f.IsExported() {
			continue
		}
		n++
	}
	return n
}

func walkFields(rt reflect.Type, prefix string, d, max int, out *[]Field) {
	if rt == nil || d > max {
		return
	}
	k := rt.Kind()
	switch k {
	case reflect.Struct:
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			if !f.IsExported() {
				continue
			}
			path := f.Name
			if prefix != "" {
				path = prefix + "." + f.Name
			}
			*out = append(*out, Field{
				Path:  path,
				Type:  f.Type.String(),
				Kind:  f.Type.Kind().String(),
				Tag:   string(f.Tag),
				Depth: d,
			})
			walkFields(f.Type, path, d+1, max, out)
		}
	case reflect.Ptr, reflect.Slice, reflect.Array:
		walkFields(rt.Elem(), prefix, d+1, max, out)
	case reflect.Map:
		walkFields(rt.Elem(), prefix, d+1, max, out)
	}
}
