// Package dump 提供 JSON / 表格 / 缩进等多种结构化打印工具，
// 便于在调试、命令行工具与日志中格式化数据。
package dump

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// JSON 以 JSON 格式返回 v。
func JSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("dump: %v", err)
	}
	return string(b)
}

// JSONIndent 以缩进 JSON 格式返回 v。
func JSONIndent(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("dump: %v", err)
	}
	return string(b)
}

// KV 将 map 渲染为 key=value 行。
func KV(m map[string]any) string {
	var b bytes.Buffer
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// 简单排序
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	for _, k := range keys {
		fmt.Fprintf(&b, "%s=%v\n", k, m[k])
	}
	return b.String()
}

// Table 渲染二维字符串数组为 ASCII 表格。
func Table(headers []string, rows [][]string) string {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	var b strings.Builder
	writeRow := func(cells []string) {
		for i, c := range cells {
			if i > 0 {
				b.WriteString("  ")
			}
			fmt.Fprintf(&b, "%-*s", widths[i], c)
		}
		b.WriteString("\n")
	}
	writeRow(headers)
	for i := range headers {
		if i > 0 {
			b.WriteString("--")
		}
		b.WriteString(strings.Repeat("-", widths[i]))
	}
	b.WriteString("\n")
	for _, row := range rows {
		writeRow(row)
	}
	return b.String()
}

// Pretty 反射地格式化任意值。
func Pretty(v any) string {
	var sb strings.Builder
	reflectValue(reflect.ValueOf(v), &sb, 0)
	return sb.String()
}

func reflectValue(rv reflect.Value, sb *strings.Builder, depth int) {
	if !rv.IsValid() {
		sb.WriteString("<invalid>")
		return
	}
	switch rv.Kind() {
	case reflect.Struct:
		sb.WriteString(rv.Type().String())
		sb.WriteString("{\n")
		for i := 0; i < rv.NumField(); i++ {
			sb.WriteString(strings.Repeat("  ", depth+1))
			sb.WriteString(rv.Type().Field(i).Name)
			sb.WriteString(": ")
			reflectValue(rv.Field(i), sb, depth+1)
			sb.WriteString("\n")
		}
		sb.WriteString(strings.Repeat("  ", depth))
		sb.WriteString("}")
	case reflect.Map, reflect.Slice, reflect.Array:
		sb.WriteString(rv.String())
	default:
		fmt.Fprintf(sb, "%v", rv.Interface())
	}
}
