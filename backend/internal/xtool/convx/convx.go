// Package convx 提供常用类型转换与字符串解析。
package convx

import (
	"errors"
	"strconv"
)

// ErrBadInt 表示整数解析失败。
var ErrBadInt = errors.New("convx: 整数解析失败")

// Atoi 把字符串转换为 int；失败返回 err。
func Atoi(s string) (int, error) {
	if s == "" {
		return 0, ErrBadInt
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return v, nil
}

// AtoiDefault 同 Atoi，失败返回 def。
func AtoiDefault(s string, def int) int {
	v, err := Atoi(s)
	if err != nil {
		return def
	}
	return v
}

// Atoi64 把字符串转换为 int64。
func Atoi64(s string) (int64, error) {
	if s == "" {
		return 0, ErrBadInt
	}
	return strconv.ParseInt(s, 10, 64)
}

// ParseFloat 解析 float。
func ParseFloat(s string) (float64, error) {
	if s == "" {
		return 0, ErrBadInt
	}
	return strconv.ParseFloat(s, 64)
}

// ParseBool 解析 bool。
func ParseBool(s string) (bool, error) {
	return strconv.ParseBool(s)
}

// Itoa 把整数转换为字符串（备用，无 sign）。
func Itoa(v int) string {
	return strconv.Itoa(v)
}
