package logx

// field.go:Field 类型与构造函数。
//
// Field 是键值对结构,用于向 Logger 附加结构化字段。

import "time"

// Field 表示一对键值。
type Field struct {
	Key   string // 字段名
	Value any    // 字段值
}

// Str 构造字符串字段。
func Str(k, v string) Field { return Field{k, v} }

// Int 构造整数字段。
func Int(k string, v int) Field { return Field{k, v} }

// Int64 构造 int64 字段。
func Int64(k string, v int64) Field { return Field{k, v} }

// Bool 构造布尔字段。
func Bool(k string, v bool) Field { return Field{k, v} }

// Dur 构造时长字段,序列化为字符串。
func Dur(k string, v time.Duration) Field { return Field{k, v.String()} }

// Err 构造 error 字段。nil error 序列化为 nil。
func Err(v error) Field {
	if v == nil {
		return Field{"error", nil}
	}
	return Field{"error", v.Error()}
}

// Time 构造时间字段,序列化为 RFC3339Nano 字符串(强制 UTC)。
func Time(k string, v time.Time) Field {
	return Field{k, v.UTC().Format(time.RFC3339Nano)}
}

// Any 构造任意类型字段。
func Any(k string, v any) Field { return Field{k, v} }
