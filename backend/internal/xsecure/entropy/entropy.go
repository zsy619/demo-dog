// Package entropy 计算字符串/字节的香农熵（用于密码强度评估）。
package entropy

import "math"

// Shannon 计算字节切片的香农熵（单位 bit）。
func Shannon(b []byte) float64 {
	if len(b) == 0 {
		return 0
	}
	counts := make(map[byte]int)
	for _, c := range b {
		counts[c]++
	}
	var e float64
	total := float64(len(b))
	for _, c := range counts {
		p := float64(c) / total
		e -= p * math.Log2(p)
	}
	return e
}

// ShannonString 是字符串便捷版本。
func ShannonString(s string) float64 {
	return Shannon([]byte(s))
}

// Strength 把熵值分类为弱/中/强。
func Strength(bits float64) string {
	switch {
	case bits < 28:
		return "very weak"
	case bits < 36:
		return "weak"
	case bits < 60:
		return "medium"
	case bits < 128:
		return "strong"
	default:
		return "very strong"
	}
}
