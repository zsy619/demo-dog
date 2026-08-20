// Package macx 提供跨算法 MAC 辅助（HMAC-SHA256/512、SHA1）。
//
// 安全性提示：
//   - HMAC-SHA256/512 是 NIST 推荐的 MAC 算法
//   - HMAC-SHA1 仅用于兼容性场景（如 TLS 1.0/1.1），新代码应避免使用
//   - Verify 使用常数时间比较
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

// ErrUnknownAlg 在算法不支持时返回。
var ErrUnknownAlg = errors.New("macx: 未知算法")

// ErrInvalidN 在 n < 1 时返回。
var ErrInvalidN = errors.New("macx: n 必须 >=1")

// ErrShortKey 在 key 长度不足时返回（< 16 字节被认为过弱）。
var ErrShortKey = errors.New("macx: 密钥过短（<16 字节）")

// MinKeyLength 是建议的最小密钥长度。
const MinKeyLength = 16

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

// SignBytes 计算 MAC 并返回原始字节。
func SignBytes(alg Alg, key, msg []byte) ([]byte, error) {
	h, err := hashFor(alg)
	if err != nil {
		return nil, err
	}
	m := hmac.New(h, key)
	m.Write(msg)
	return m.Sum(nil), nil
}

// Verify 常时间比较 MAC（hex 输入）。
func Verify(alg Alg, key, msg []byte, expectedHex string) (bool, error) {
	got, err := Sign(alg, key, msg)
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(expectedHex)) == 1, nil
}

// VerifyBytes 常时间比较 MAC（原始字节）。
func VerifyBytes(alg Alg, key, msg, expected []byte) (bool, error) {
	got, err := SignBytes(alg, key, msg)
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(got, expected) == 1, nil
}

// Truncated 计算 MAC 并返回截断到 n 字节的十六进制。
// n > hash 输出大小时自动截到实际大小。
func Truncated(alg Alg, key, msg []byte, n int) (string, error) {
	if n < 1 {
		return "", ErrInvalidN
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

// CheckKey 校验 key 强度（不强制但提供建议）。
// weakKeys 为 true 时，短密钥返回 ErrShortKey。
func CheckKey(key []byte, weakKeys bool) error {
	if weakKeys && len(key) < MinKeyLength {
		return ErrShortKey
	}
	return nil
}

// Size 返回算法输出字节数。
func Size(alg Alg) (int, error) {
	switch alg {
	case SHA256:
		return sha256.Size, nil
	case SHA512:
		return sha512.Size, nil
	case SHA1:
		return sha1.Size, nil
	}
	return 0, ErrUnknownAlg
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
	return nil, ErrUnknownAlg
}
