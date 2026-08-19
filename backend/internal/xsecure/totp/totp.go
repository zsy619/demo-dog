// Package totp 提供基于时间的一次性密码（RFC 6238）实现。
// 使用 HMAC-SHA1 作为默认算法，可在 Verify 中调整容差窗口。
package totp

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"hash"
	"strings"
	"time"
)

// Algorithm 是 HMAC 算法枚举。
type Algorithm int

const (
	SHA1 Algorithm = iota
	SHA256
	SHA512
)

// DefaultDigits 是默认口令位数。
const DefaultDigits = 6

// DefaultPeriod 是默认时间步长（30 秒）。
const DefaultPeriod = 30

// Config 是 TOTP 配置。
type Config struct {
	Secret    []byte
	Digits    int
	Period    int
	Algorithm Algorithm
	T0        int64 // 时间起点
}

// Defaults 应用默认配置。
func (c *Config) Defaults() {
	if c.Digits <= 0 {
		c.Digits = DefaultDigits
	}
	if c.Period <= 0 {
		c.Period = DefaultPeriod
	}
}

// Generate 返回指定时间的口令。
func Generate(c Config, t time.Time) string {
	c.Defaults()
	if c.T0 == 0 {
		c.T0 = 0
	}
	counter := uint64(t.Unix()-c.T0) / uint64(c.Period)
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)
	mac := hmac.New(hashFunc(c.Algorithm), c.Secret)
	mac.Write(buf[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	code := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	mod := uint32(1)
	for i := 0; i < c.Digits; i++ {
		mod *= 10
	}
	return leftPad(int(code%mod), c.Digits)
}

// Verify 验证口令并允许 window 步长容差。
func Verify(c Config, t time.Time, passcode string, window int) bool {
	c.Defaults()
	passcode = strings.TrimSpace(passcode)
	if len(passcode) != c.Digits {
		return false
	}
	for i := -window; i <= window; i++ {
		t2 := t.Add(time.Duration(i) * time.Duration(c.Period) * time.Second)
		if Generate(c, t2) == passcode {
			return true
		}
	}
	return false
}

func hashFunc(a Algorithm) func() hash.Hash {
	switch a {
	case SHA256:
		return sha256.New
	case SHA512:
		return sha512.New
	default:
		return sha1.New
	}
}

func leftPad(n, w int) string {
	s := ""
	if n == 0 {
		s = "0"
	} else {
		for n > 0 {
			s = string(rune('0'+n%10)) + s
			n /= 10
		}
	}
	for len(s) < w {
		s = "0" + s
	}
	return s
}
