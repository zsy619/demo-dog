// Package strutilx 提供字符串分割、截断、填充等辅助函数。
package strutilx

import "strings"

// Truncate 把 s 截断到 max 字节（按字符边界）。
func Truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

// PadLeft 用 pad 把 s 左填充到 width。
func PadLeft(s, pad string, width int) string {
	if len(s) >= width {
		return s
	}
	if pad == "" {
		pad = " "
	}
	for len(s)+len(pad) <= width {
		s = pad + s
	}
	if len(s) < width {
		s = strings.Repeat(pad, width-len(s)) + s
	}
	return s
}

// PadRight 用 pad 把 s 右填充到 width。
func PadRight(s, pad string, width int) string {
	if len(s) >= width {
		return s
	}
	if pad == "" {
		pad = " "
	}
	for len(s) < width {
		s += pad
		if len(s) > width {
			s = s[:width]
		}
	}
	return s
}

// SplitLines 把 s 按 \n 拆成多行。
func SplitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// Words 把 s 按空白拆为单词列表。
func Words(s string) []string {
	return strings.Fields(s)
}

// Reverse 字符串反转（按 rune）。
func Reverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

// ContainsAny 判断 s 是否包含任意 sub。
func ContainsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// CountOccurrences 统计 sub 在 s 中出现的次数。
func CountOccurrences(s, sub string) int {
	if sub == "" {
		return 0
	}
	return strings.Count(s, sub)
}
