// Package printx 提供格式化输出辅助：表格、行字段、列表。
package printx

import (
	"fmt"
	"io"
	"strings"
)

// KV 以 key=value 形式打印对象。
func KV(w io.Writer, kv map[string]any) {
	for k, v := range kv {
		fmt.Fprintf(w, "%s=%v\n", k, v)
	}
}

// Table 把二维字符串切片以对齐方式输出。
func Table(w io.Writer, rows [][]string) {
	if len(rows) == 0 {
		return
	}
	cols := len(rows[0])
	widths := make([]int, cols)
	for _, row := range rows {
		for i, cell := range row {
			if i >= cols {
				continue
			}
			if l := len(cell); l > widths[i] {
				widths[i] = l
			}
		}
	}
	for _, row := range rows {
		parts := make([]string, cols)
		for i := 0; i < cols; i++ {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			parts[i] = padRight(cell, widths[i])
		}
		fmt.Fprintln(w, strings.Join(parts, "  "))
	}
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

// List 把切片以编号形式输出。
func List(w io.Writer, items []string) {
	for i, it := range items {
		fmt.Fprintf(w, "%d. %s\n", i+1, it)
	}
}

// Section 输出带分隔线的标题。
func Section(w io.Writer, title string) {
	fmt.Fprintln(w, "==", title, "==")
}

// Indent 在每行前增加指定缩进。
func Indent(w io.Writer, s string, level int) {
	pad := strings.Repeat("  ", level)
	for _, line := range strings.Split(s, "\n") {
		fmt.Fprintln(w, pad+line)
	}
}

// Bytes 友好地格式化字节数。
func Bytes(n int64) string {
	const k = 1024
	switch {
	case n < k:
		return fmt.Sprintf("%dB", n)
	case n < k*k:
		return fmt.Sprintf("%.1fKB", float64(n)/k)
	case n < k*k*k:
		return fmt.Sprintf("%.1fMB", float64(n)/(k*k))
	default:
		return fmt.Sprintf("%.1fGB", float64(n)/(k*k*k))
	}
}

// Duration 友好地格式化纳秒数。
func Duration(ns int64) string {
	switch {
	case ns < 1000:
		return fmt.Sprintf("%dns", ns)
	case ns < 1000*1000:
		return fmt.Sprintf("%.1fus", float64(ns)/1000)
	case ns < 1000*1000*1000:
		return fmt.Sprintf("%.1fms", float64(ns)/(1000*1000))
	default:
		return fmt.Sprintf("%.2fs", float64(ns)/(1000*1000*1000))
	}
}
