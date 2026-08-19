// Package encodex 封装 base64、base32、base58、hex 之间的互转。
package encodex

import (
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
)

// Base64Std 用标准 base64 编码。
func Base64Std(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

// Base64URL 用 URL 友好 base64 编码。
func Base64URL(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

// FromBase64Std 解码标准 base64。
func FromBase64Std(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// FromBase64URL 解码 URL base64。
func FromBase64URL(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

// Hex 用十六进制编码。
func Hex(b []byte) string {
	return hex.EncodeToString(b)
}

// FromHex 解码十六进制。
func FromHex(s string) ([]byte, error) {
	return hex.DecodeString(s)
}

// Base32Std 用标准 base32 编码。
func Base32Std(b []byte) string {
	return base32.StdEncoding.EncodeToString(b)
}

// FromBase32Std 解码标准 base32。
func FromBase32Std(s string) ([]byte, error) {
	return base32.StdEncoding.DecodeString(s)
}

// Base58 用比特币字母表做 base58 编码。
func Base58(b []byte) string {
	alphabet := "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	if len(b) == 0 {
		return ""
	}
	zeros := 0
	for zeros < len(b) && b[zeros] == 0 {
		zeros++
	}
	digits := make([]byte, 0, len(b)*138/100+1)
	for _, by := range b {
		carry := uint64(by)
		for i := 0; i < len(digits) && carry != 0; i++ {
			carry += uint64(digits[i]) << 8
			digits[i] = byte(carry % 58)
			carry /= 58
		}
		for carry != 0 {
			digits = append(digits, byte(carry%58))
			carry /= 58
		}
	}
	out := strings.Repeat("1", zeros)
	for i := len(digits) - 1; i >= 0; i-- {
		out += string(alphabet[digits[i]])
	}
	return out
}

// FromBase58 解码 base58。
func FromBase58(s string) ([]byte, error) {
	alphabet := "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	if s == "" {
		return nil, nil
	}
	idx := make(map[byte]int, 58)
	for i := 0; i < len(alphabet); i++ {
		idx[alphabet[i]] = i
	}
	zeros := 0
	for zeros < len(s) && s[zeros] == '1' {
		zeros++
	}
	digits := make([]byte, 0, len(s)*733/1000+1)
	for i := zeros; i < len(s); i++ {
		v, ok := idx[s[i]]
		if !ok {
			return nil, ErrBadInput
		}
		carry := uint32(v)
		for j := 0; j < len(digits); j++ {
			carry += uint32(digits[j]) * 58
			digits[j] = byte(carry & 0xff)
			carry >>= 8
		}
		for carry != 0 {
			digits = append(digits, byte(carry&0xff))
			carry >>= 8
		}
	}
	out := make([]byte, zeros+len(digits))
	for i := 0; i < len(digits); i++ {
		out[zeros+len(digits)-1-i] = digits[i]
	}
	return out, nil
}

// ErrBadInput 在通用转换输入非法时返回。
var ErrBadInput = errors.New("encodex: 非法输入")
