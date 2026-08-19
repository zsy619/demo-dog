// Package uuid 提供轻量的 UUID v4 生成与字符串/字节转换工具，
// 基于 crypto/rand，无第三方依赖。
package uuid

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
)

// Size 是 UUID 字节长度。
const Size = 16

// ErrShort 在输入过短时返回。
var ErrShort = errors.New("uuid: 输入过短")

// New 返回一个 v4 UUID 字符串（含连字符）。
func New() (string, error) {
	b, err := Bytes()
	if err != nil {
		return "", err
	}
	return Format(b), nil
}

// MustNew 在随机源失败时 panic；适合测试与短脚本。
func MustNew() string {
	s, err := New()
	if err != nil {
		panic(err)
	}
	return s
}

// Bytes 返回一个新生成的 16 字节 UUID（RFC 4122 v4）。
func Bytes() ([]byte, error) {
	b := make([]byte, Size)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant
	return b, nil
}

// Format 把 16 字节转换为 8-4-4-4-12 形式。
func Format(b []byte) string {
	if len(b) != Size {
		return ""
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Parse 把 UUID 字符串解析为字节。
func Parse(s string) ([]byte, error) {
	if len(s) != 36 {
		return nil, ErrShort
	}
	var b [Size]byte
	hexStr := s[0:8] + s[9:13] + s[14:18] + s[19:23] + s[24:36]
	dst := make([]byte, hex.DecodedLen(len(hexStr)))
	n, err := hex.Decode(dst, []byte(hexStr))
	if err != nil {
		return nil, err
	}
	if n != Size {
		return nil, ErrShort
	}
	copy(b[:], dst)
	return b[:], nil
}

// Version 返回 UUID 字节的版本号。
func Version(b []byte) int {
	if len(b) < Size {
		return 0
	}
	return int(b[6] >> 4)
}

// IsValid 校验字符串是否为合法 UUID。
func IsValid(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !isHex(byte(c)) {
				return false
			}
		}
	}
	return true
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}
