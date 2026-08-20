// Package strutil 提供常用字符串处理辅助函数。
package strutil

import (
	"strings"
	"unicode"
)

// IsEmpty 返回 s 是否为空或仅含空白。
func IsEmpty(s string) bool {
	for _, r := range s {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

// Truncate 把 s 截断为最长 n 个字符（rune）。
func Truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if n >= len([]rune(s)) {
		return s
	}
	return string([]rune(s)[:n])
}

// CamelCase 把 s 转成 camelCase。
func CamelCase(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	if len(parts) == 0 {
		return ""
	}
	var b strings.Builder
	for i, p := range parts {
		if i == 0 {
			b.WriteString(strings.ToLower(p))
		} else {
			r := []rune(p)
			r[0] = unicode.ToUpper(r[0])
			b.WriteString(string(r))
		}
	}
	return b.String()
}

// SnakeCase 把 s 转成 snake_case。
func SnakeCase(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	return strings.ToLower(strings.Join(parts, "_"))
}

// ContainsAny 返回 s 是否包含任一 sub。
func ContainsAny(s string, sub ...string) bool {
	for _, x := range sub {
		if strings.Contains(s, x) {
			return true
		}
	}
	return false
}

// Reverse 反转字符串。
func Reverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

// WordCount 简单分词计数（按空白）。
func WordCount(s string) int {
	return len(strings.Fields(s))
}

// MaskEmail 隐藏邮箱中间部分。
func MaskEmail(e string) string {
	at := strings.IndexByte(e, '@')
	if at <= 1 {
		return e
	}
	return e[:1] + strings.Repeat("*", at-1) + e[at:]
}

// PadLeft 用 pad 把 s 左填充到 width；
// width 为字节数。pad 为空时默认为单空格。
func PadLeft(s, pad string, width int) string {
	n := len(s)
	if n >= width {
		return s
	}
	if pad == "" {
		pad = " "
	}
	pl := len(pad)
	if pl == 0 {
		return s
	}
	// 计算需要重复的次数 + 余量
	diff := width - n
	repeat := diff / pl
	remain := diff - repeat*pl
	// Build: pad[:remain] + pad*repeat + s
	var b strings.Builder
	b.Grow(width + pl)
	if remain > 0 {
		b.WriteString(pad[:remain])
	}
	for i := 0; i < repeat; i++ {
		b.WriteString(pad)
	}
	b.WriteString(s)
	return b.String()
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
func Words(s string) []string { return strings.Fields(s) }

// CountOccurrences 统计 sub 在 s 中出现的次数。
func CountOccurrences(s, sub string) int {
	if sub == "" {
		return 0
	}
	return strings.Count(s, sub)
}
