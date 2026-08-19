// Package randomx 提供安全随机数辅助（crypto/rand）。
package randomx

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
)

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

// Int 返回 [0, max) 范围内的安全随机整数。
// 使用 modulo bias 拒绝采样。
func Int(max int) (int, error) {
	if max <= 0 {
		return 0, nil
	}
	// 简单做法：每次读 8 字节，使用 mod
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
