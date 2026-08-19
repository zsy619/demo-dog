// Package reflection 提供类型名/字段遍历等辅助。
package reflection

import (
	"reflect"
)

// TypeName 返回任意值的类型名（包路径去掉）。
func TypeName(v any) string {
	t := reflect.TypeOf(v)
	if t == nil {
		return ""
	}
	return t.String()
}

// FieldNames 返回结构体导出字段名列表。
func FieldNames(v any) []string {
	t := reflect.TypeOf(v)
	if t == nil || t.Kind() != reflect.Struct {
		return nil
	}
	out := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.IsExported() {
			out = append(out, f.Name)
		}
	}
	return out
}

// IsZero 返回 v 是否是其类型的零值。
func IsZero(v any) bool {
	if v == nil {
		return true
	}
	return reflect.ValueOf(v).IsZero()
}

// Copy 把 src 的字段浅拷贝到 dst，要求两者同类型或结构兼容。
func Copy(dst, src any) error {
	dv := reflect.ValueOf(dst)
	sv := reflect.ValueOf(src)
	if dv.Kind() != reflect.Pointer || sv.Kind() != reflect.Pointer {
		return errBadPointer
	}
	dv = dv.Elem()
	sv = sv.Elem()
	if !dv.CanSet() || dv.Type() != sv.Type() {
		return errTypeMismatch
	}
	dv.Set(sv)
	return nil
}

type stringError string

func (s stringError) Error() string { return string(s) }

var (
	errBadPointer   = stringError("reflection: 必须传指针")
	errTypeMismatch = stringError("reflection: 类型不匹配")
)
