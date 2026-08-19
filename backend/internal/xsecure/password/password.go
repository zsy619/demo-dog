// Package password 提供密码强度评估与哈希（PBKDF2-SHA256）。
package password

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Strength 是评估的强度等级。
type Strength int

const (
	StrengthWeak Strength = iota
	StrengthFair
	StrengthGood
	StrengthStrong
)

// String 返回强度名称。
func (s Strength) String() string {
	switch s {
	case StrengthWeak:
		return "weak"
	case StrengthFair:
		return "fair"
	case StrengthGood:
		return "good"
	case StrengthStrong:
		return "strong"
	}
	return "unknown"
}

// Score 评估强度（基于长度+字符多样性+黑名单）。
func Score(p string) Strength {
	score := 0
	var hasLower, hasUpper, hasDigit, hasSym bool
	for _, r := range p {
		switch {
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSym = true
		}
	}
	if hasLower {
		score++
	}
	if hasUpper {
		score++
	}
	if hasDigit {
		score++
	}
	if hasSym {
		score++
	}
	if len(p) >= 12 {
		score += 2
	} else if len(p) >= 8 {
		score++
	}
	if isCommon(p) {
		score -= 2
	}
	switch {
	case score <= 1:
		return StrengthWeak
	case score <= 3:
		return StrengthFair
	case score <= 5:
		return StrengthGood
	default:
		return StrengthStrong
	}
}

var common = []string{
	"password", "123456", "qwerty", "letmein", "admin", "welcome", "abc123",
}

func isCommon(p string) bool {
	lp := strings.ToLower(p)
	for _, c := range common {
		if lp == c {
			return true
		}
	}
	return false
}

// Hash 是 PBKDF2-SHA256 编码后的字符串（salt:hash）。
func Hash(password string, iter int) (string, error) {
	if iter < 1000 {
		iter = 10000
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	h := pbkdf2(password, salt, iter, 32)
	return fmt.Sprintf("%d:%s:%s", iter, base64.RawURLEncoding.EncodeToString(salt), base64.RawURLEncoding.EncodeToString(h)), nil
}

// Verify 校验明文密码与已编码字符串。
func Verify(encoded, password string) (bool, error) {
	parts := strings.Split(encoded, ":")
	if len(parts) != 3 {
		return false, errors.New("password: 编码格式错误")
	}
	iter, err := strconv.Atoi(parts[0])
	if err != nil {
		return false, err
	}
	salt, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false, err
	}
	expected, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return false, err
	}
	got := pbkdf2(password, salt, iter, 32)
	return subtle.ConstantTimeCompare(got, expected) == 1, nil
}

// pbkdf2 实现 PBKDF2-HMAC-SHA256。
func pbkdf2(password string, salt []byte, iter, keyLen int) []byte {
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
		u := prf([]byte(password), block)
		t := make([]byte, hLen)
		copy(t, u)
		for j := 1; j < iter; j++ {
			u = prf([]byte(password), u)
			for k := 0; k < hLen; k++ {
				t[k] ^= u[k]
			}
		}
		out = append(out, t...)
	}
	return out[:keyLen]
}
