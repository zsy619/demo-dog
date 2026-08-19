// Package randomx 提供随机字符串/数字/UUID 等生成。
package randomx

import (
	"crypto/rand"
	"encoding/hex"
)

const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// String 返回 n 字节随机字符串。
func String(n int) string {
	if n < 1 {
		n = 8
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	for i, v := range b {
		b[i] = alphabet[int(v)%len(alphabet)]
	}
	return string(b)
}

// Hex 返回 n 字节随机十六进制字符串。
func Hex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

// Bytes 返回 n 字节随机字节。
func Bytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil
	}
	return b
}

// Int 返回 [0, n) 的随机整数（基于 crypto/rand 截断）。
func Int(n int) int {
	if n < 1 {
		return 0
	}
	b := Bytes(8)
	v := int(b[0]) | int(b[1])<<8 | int(b[2])<<16 | int(b[3])<<24 |
		int(b[4])<<32 | int(b[5])<<40 | int(b[6])<<48 | int(b[7])<<56
	if v < 0 {
		v = -v
	}
	return v % n
}
