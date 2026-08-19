// Package macx 提供跨算法 MAC 辅助（HMAC、CMAC 简化版）。
package macx

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"hash"
)

// Alg 是 MAC 算法标识。
type Alg string

const (
	SHA256 Alg = "sha256"
	SHA512 Alg = "sha512"
	SHA1   Alg = "sha1"
)

// Sign 计算 MAC 并返回十六进制字符串。
func Sign(alg Alg, key, msg []byte) (string, error) {
	h, err := hashFor(alg)
	if err != nil {
		return "", err
	}
	m := hmac.New(h, key)
	m.Write(msg)
	return hex.EncodeToString(m.Sum(nil)), nil
}

// Verify 常时间比较 MAC。
func Verify(alg Alg, key, msg []byte, expected string) (bool, error) {
	got, err := Sign(alg, key, msg)
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1, nil
}

// Truncated 计算 MAC 并返回截断到 n 字节的十六进制。
func Truncated(alg Alg, key, msg []byte, n int) (string, error) {
	if n < 1 {
		return "", errors.New("macx: n 必须 >=1")
	}
	h, err := hashFor(alg)
	if err != nil {
		return "", err
	}
	m := hmac.New(h, key)
	m.Write(msg)
	sum := m.Sum(nil)
	if n > len(sum) {
		n = len(sum)
	}
	return hex.EncodeToString(sum[:n]), nil
}

func hashFor(alg Alg) (func() hash.Hash, error) {
	switch alg {
	case SHA256:
		return sha256.New, nil
	case SHA512:
		return sha512.New, nil
	case SHA1:
		return sha1.New, nil
	}
	return nil, errors.New("macx: 未知算法")
}
