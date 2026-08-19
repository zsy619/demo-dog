// Package hashing 提供 PBKDF2 风格的口令哈希与常量时间比较工具。
// 它使用 stdlib 的 crypto/sha256 + crypto/subtle，无第三方依赖。
package hashing

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
)

const (
	saltLen  = 16
	keyLen   = 32
	minIters = 10000
)

// Hash 以 pbkdf2 风格将口令哈希为 $v1$it$salt$hash 字符串。
func Hash(password string, iters int) (string, error) {
	if iters < minIters {
		iters = minIters
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := pbkdf2(password, salt, iters, keyLen)
	return fmtFormat(iters, salt, key), nil
}

// Verify 比较明文口令与已哈希字符串。
func Verify(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 5 || parts[1] != "v1" {
		return false, errors.New("hashing: 格式非法")
	}
	iters, err := strconv.Atoi(parts[2])
	if err != nil {
		return false, err
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false, err
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}
	got := pbkdf2(password, salt, iters, len(want))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

func fmtFormat(iters int, salt, key []byte) string {
	return "$v1$" + strconv.Itoa(iters) + "$" +
		base64.RawStdEncoding.EncodeToString(salt) + "$" +
		base64.RawStdEncoding.EncodeToString(key)
}

func pbkdf2(password string, salt []byte, iters, length int) []byte {
	out := make([]byte, length)
	prf := sha256.New
	h := prf()
	h.Write([]byte(password))
	h.Write(salt)
	u := h.Sum(nil)
	t := make([]byte, len(u))
	copy(t, u)
	filled := 0
	for filled < length {
		for i := 1; i < iters; i++ {
			h.Reset()
			h.Write(t)
			u = h.Sum(u[:0])
			for j := range t {
				t[j] ^= u[j]
			}
		}
		n := copy(out[filled:], t)
		filled += n
	}
	return out
}

// Equal 提供常量时间字符串比较。
func Equal(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// RandomToken 生成指定字节长度的随机 token，并进行 base64 编码。
func RandomToken(n int) (string, error) {
	if n <= 0 {
		n = 32
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
