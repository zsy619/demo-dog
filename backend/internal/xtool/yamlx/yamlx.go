// Package yamlx 提供一个最小化的 YAML 序列化器，
// 支持 map[string]any、[]any 与基础类型。
// 仅用于内部配置导出，避免引入第三方 YAML 库。
package yamlx

import (
	"fmt"
	"io"
	"strings"
)

// Marshal 把任意 v 序列化为 YAML 写入 w。
func Marshal(w io.Writer, v any) error {
	var sb strings.Builder
	write(&sb, v, 0, "")
	_, err := io.WriteString(w, sb.String())
	return err
}

func indent(level int) string {
	return strings.Repeat("  ", level)
}

func write(sb *strings.Builder, v any, level int, key string) {
	prefix := indent(level)
	if key != "" {
		prefix += key + ":"
	}
	switch t := v.(type) {
	case nil:
		sb.WriteString(prefix)
		sb.WriteString(" null\n")
	case bool:
		sb.WriteString(fmt.Sprintf("%s %t\n", prefix, t))
	case int:
		sb.WriteString(fmt.Sprintf("%s %d\n", prefix, t))
	case int64:
		sb.WriteString(fmt.Sprintf("%s %d\n", prefix, t))
	case float64:
		sb.WriteString(fmt.Sprintf("%s %v\n", prefix, t))
	case string:
		if strings.ContainsAny(t, "\n:#") {
			sb.WriteString(fmt.Sprintf("%s |\n", prefix))
			for _, line := range strings.Split(t, "\n") {
				sb.WriteString(indent(level+1) + line + "\n")
			}
		} else {
			sb.WriteString(fmt.Sprintf("%s %s\n", prefix, t))
		}
	case []any:
		if len(t) == 0 {
			sb.WriteString(prefix + " []\n")
			return
		}
		for _, it := range t {
			write(sb, it, level+1, "-")
		}
	case map[string]any:
		if len(t) == 0 {
			sb.WriteString(prefix + " {}\n")
			return
		}
		if key == "" {
			for k, v := range t {
				write(sb, v, level, k)
			}
		} else {
			sb.WriteString(prefix + "\n")
			for k, v := range t {
				write(sb, v, level+1, k)
			}
		}
	default:
		sb.WriteString(fmt.Sprintf("%s %v\n", prefix, v))
	}
}
