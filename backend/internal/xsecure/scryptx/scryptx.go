// Package scryptx 提供基于 PBKDF2-HMAC-SHA256 的密码哈希。
// 命名为 scryptx 以纪念接口形状，可由调用方替换成真正的 scrypt。
package scryptx

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
)

const (
	defaultIter = 100000
	defaultLen  = 32
	saltLen     = 16
)

// Hash 用 PBKDF2-SHA256 哈希密码。
func Hash(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := pbkdf2([]byte(password), salt, defaultIter, defaultLen)
	return "scryptx$" + strconv.Itoa(defaultIter) + "$" +
		base64.RawURLEncoding.EncodeToString(salt) + "$" +
		base64.RawURLEncoding.EncodeToString(key), nil
}

// Verify 校验密码。
func Verify(encoded, password string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "scryptx" {
		return false, errors.New("scryptx: 格式错误")
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil {
		return false, err
	}
	salt, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return false, err
	}
	expected, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil {
		return false, err
	}
	got := pbkdf2([]byte(password), salt, iter, len(expected))
	return subtle.ConstantTimeCompare(got, expected) == 1, nil
}

// pbkdf2 实现 PBKDF2-HMAC-SHA256。
func pbkdf2(password, salt []byte, iter, keyLen int) []byte {
	prf := func(key, msg []byte) []byte {
		m := hmac.New(sha256.New, key)
		m.Write(msg)
		return m.Sum(nil)
	}
	hLen := 32
	n := (keyLen + hLen - 1) / hLen
	out := make([]byte, 0, n*hLen)
	for i := 1; i <= n; i++ {
		block := make([]byte, len(salt)+4)
		copy(block, salt)
		block[len(salt)] = byte(i >> 24)
		block[len(salt)+1] = byte(i >> 16)
		block[len(salt)+2] = byte(i >> 8)
		block[len(salt)+3] = byte(i)
		u := prf(password, block)
		t := make([]byte, hLen)
		copy(t, u)
		for j := 1; j < iter; j++ {
			u = prf(password, u)
			for k := 0; k < hLen; k++ {
				t[k] ^= u[k]
			}
		}
		out = append(out, t...)
	}
	return out[:keyLen]
}
