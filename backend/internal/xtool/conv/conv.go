// Package conv 提供常见类型之间的安全转换辅助：
// 支持字符串/数字、interface 转具体类型、空值兜底等。
package conv

import (
	"fmt"
	"reflect"
	"strconv"
)

// ToString 将任意值转为字符串。
func ToString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	case error:
		return t.Error()
	case bool:
		if t {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatFloat(t, 'g', -1, 64)
	}
	return fmt.Sprintf("%v", v)
}

// ToInt 将任意可转换值转为 int。
func ToInt(v any) (int, error) {
	switch t := v.(type) {
	case int:
		return t, nil
	case int32:
		return int(t), nil
	case int64:
		return int(t), nil
	case float64:
		return int(t), nil
	case string:
		return strconv.Atoi(t)
	case []byte:
		return strconv.Atoi(string(t))
	case bool:
		if t {
			return 1, nil
		}
		return 0, nil
	}
	return 0, fmt.Errorf("conv: 无法转为 int: %T", v)
}

// MustToInt 与 ToInt 类似，但在失败时返回 0。
func MustToInt(v any) int {
	n, _ := ToInt(v)
	return n
}

// ToBool 将任意可转换值转为 bool。
func ToBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case int:
		return t != 0
	case int64:
		return t != 0
	case float64:
		return t != 0
	case string:
		b, _ := strconv.ParseBool(t)
		return b
	}
	return false
}

// ToFloat64 转 float64。
func ToFloat64(v any) (float64, error) {
	switch t := v.(type) {
	case float64:
		return t, nil
	case float32:
		return float64(t), nil
	case int:
		return float64(t), nil
	case int64:
		return float64(t), nil
	case string:
		return strconv.ParseFloat(t, 64)
	}
	return 0, fmt.Errorf("conv: 无法转为 float64: %T", v)
}

// ToMap 反射地把 struct 转为 map[string]any。
func ToMap(v any) map[string]any {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Struct {
		return nil
	}
	out := make(map[string]any, rv.NumField())
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		if !rt.Field(i).IsExported() {
			continue
		}
		out[rt.Field(i).Name] = rv.Field(i).Interface()
	}
	return out
}

// OrDefault 在 v 为零值时返回 def。
func OrDefault[T comparable](v, def T) T {
	var zero T
	if v == zero {
		return def
	}
	return v
}
