// Package randomx 提供基于 crypto/rand 的安全随机数工具。
package randomx

import (
	"crypto/rand"
	"encoding/hex"
)

// Bytes 返回 n 字节随机数据。
func Bytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

// Hex 返回 2n 长度的 hex 字符串。
func Hex(n int) (string, error) {
	b, err := Bytes(n)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Int63 返回 [0, max) 内的随机整数。
func Int63(max int64) (int64, error) {
	if max <= 0 {
		return 0, nil
	}
	// 取 8 字节并 mod max
	b, err := Bytes(8)
	if err != nil {
		return 0, err
	}
	v := int64(0)
	for i := 0; i < 8; i++ {
		v = (v << 8) | int64(b[i])
	}
	if v < 0 {
		v = -v
	}
	return v % max, nil
}

// String 返回指定长度的随机字母数字串。
func String(n int) (string, error) {
	const alpha = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		idx, err := Int63(int64(len(alpha)))
		if err != nil {
			return "", err
		}
		out[i] = alpha[idx]
	}
	return string(out), nil
}
