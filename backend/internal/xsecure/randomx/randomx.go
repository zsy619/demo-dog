// Package randomx 提供基于 crypto/rand 的安全随机数辅助：
// 字节、十六进制、Base64、URL-safe token、随机整数与字母数字字符串。
package randomx

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
)

const alphanumeric = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// Bytes 返回 n 字节安全随机数。
func Bytes(n int) ([]byte, error) {
	if n < 1 {
		return nil, nil
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

// Hex 返回 n 字节随机数的 hex 字符串。
func Hex(n int) (string, error) {
	b, err := Bytes(n)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Base64 返回 n 字节随机数的 base64 字符串。
func Base64(n int) (string, error) {
	b, err := Bytes(n)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// Token 返回 URL 安全的随机 token（默认 32 字节）。
func Token(n ...int) (string, error) {
	l := 32
	if len(n) > 0 && n[0] > 0 {
		l = n[0]
	}
	b, err := Bytes(l)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Int 返回 [0, max) 范围内的安全随机整数（拒绝采样）。
func Int(max int) (int, error) {
	if max <= 0 {
		return 0, nil
	}
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return 0, err
	}
	var v uint64
	for _, x := range b {
		v = v<<8 | uint64(x)
	}
	return int(v % uint64(max)), nil
}

// String 返回 n 字节随机字母数字字符串（非密码学强，但适合 ID 场景）。
func String(n int) string {
	if n < 1 {
		n = 8
	}
	b, _ := Bytes(n)
	if b == nil {
		return ""
	}
	for i, v := range b {
		b[i] = alphanumeric[int(v)%len(alphanumeric)]
	}
	return string(b)
}
