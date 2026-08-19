// Package diff 对 map[string]any/结构体的 JSON 表示做差分计算，
// 用于审计、同步和审计日志比对场景。
package diff

import (
	"encoding/json"
	"reflect"
	"sort"
)

// Op 描述差异操作类型。
type Op string

const (
	OpAdd    Op = "add"
	OpRemove Op = "remove"
	OpChange Op = "change"
)

// Change 描述一个字段的差异。
type Change struct {
	Path string `json:"path"`
	Op   Op     `json:"op"`
	Old  any    `json:"old,omitempty"`
	New  any    `json:"new,omitempty"`
}

// Diff 比较 a 与 b（任意类型）并返回按路径排序的差异列表。
func Diff(a, b any) []Change {
	ma := normalize(a)
	mb := normalize(b)
	var out []Change
	walk("", ma, mb, &out)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func normalize(v any) any {
	if v == nil {
		return nil
	}
	switch reflect.ValueOf(v).Kind() {
	case reflect.Struct, reflect.Map, reflect.Slice, reflect.Array:
		b, err := json.Marshal(v)
		if err != nil {
			return v
		}
		var out any
		if err := json.Unmarshal(b, &out); err != nil {
			return v
		}
		return out
	default:
		return v
	}
}

func walk(path string, a, b any, out *[]Change) {
	if reflect.DeepEqual(a, b) {
		return
	}
	ma, aok := a.(map[string]any)
	mb, bok := b.(map[string]any)
	if aok && bok {
		keys := map[string]bool{}
		for k := range ma {
			keys[k] = true
		}
		for k := range mb {
			keys[k] = true
		}
		sorted := make([]string, 0, len(keys))
		for k := range keys {
			sorted = append(sorted, k)
		}
		sort.Strings(sorted)
		for _, k := range sorted {
			child := path
			if child == "" {
				child = k
			} else {
				child = child + "." + k
			}
			walk(child, ma[k], mb[k], out)
		}
		return
	}
	op := OpChange
	if a == nil {
		op = OpAdd
	} else if b == nil {
		op = OpRemove
	}
	*out = append(*out, Change{Path: path, Op: op, Old: a, New: b})
}

// HasChanges 报告差异列表是否非空。
func HasChanges(c []Change) bool {
	return len(c) > 0
}
