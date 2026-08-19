// Package random 提供基于 crypto/rand 的安全随机辅助函数。
package random

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
)

// ErrSource 在系统随机源失败时返回。
var ErrSource = errors.New("random: 随机源失败")

// Bytes 返回 n 字节随机数据。
func Bytes(n int) ([]byte, error) {
	if n <= 0 {
		return nil, nil
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

// Hex 返回 n 字节随机数据的十六进制编码。
func Hex(n int) (string, error) {
	b, err := Bytes(n)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Base64 返回 n 字节随机数据的 URL base64 编码。
func Base64(n int) (string, error) {
	b, err := Bytes(n)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Int 返回 [0, max) 范围内的随机整数。
func Int(max int) (int, error) {
	if max <= 0 {
		return 0, errors.New("random: max 应 > 0")
	}
	b, err := Bytes(8)
	if err != nil {
		return 0, err
	}
	v := int(b[0]) | int(b[1])<<8 | int(b[2])<<16 | int(b[3])<<24 |
		int(b[4])<<32 | int(b[5])<<40 | int(b[6])<<48 | int(b[7])<<56
	if v < 0 {
		v = -v
	}
	return v % max, nil
}

// Choice 在切片中随机选择一个元素。
func Choice[T any](s []T) (T, error) {
	var zero T
	if len(s) == 0 {
		return zero, errors.New("random: 空切片")
	}
	i, err := Int(len(s))
	if err != nil {
		return zero, err
	}
	return s[i], nil
}

// Token 是一个 32 字节的 hex 字符串快捷方式。
func Token() (string, error) { return Hex(32) }
