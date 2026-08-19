// Package jwt 提供一个零依赖的轻量 JWT（JSON Web Token）实现：
// 默认 HS256 签名，支持自定义 header/payload 字段。
package jwt

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Algorithm 是签名算法。
type Algorithm int

const (
	HS256 Algorithm = iota
)

// Claims 是 JWT payload。
type Claims map[string]any

// Errors
var (
	ErrBadFormat    = errors.New("jwt: 格式错误")
	ErrBadSignature = errors.New("jwt: 签名错误")
	ErrExpired      = errors.New("jwt: 已过期")
)

// Sign 用 secret 签发 token，alg 指定算法。
func Sign(alg Algorithm, secret []byte, claims Claims) (string, error) {
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	if alg != HS256 {
		return "", fmt.Errorf("jwt: 不支持的算法")
	}
	hb, _ := json.Marshal(header)
	pb, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	h := base64.RawURLEncoding.EncodeToString(hb)
	p := base64.RawURLEncoding.EncodeToString(pb)
	signing := h + "." + p
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signing))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return signing + "." + sig, nil
}

// Verify 校验签名并返回 claims。
func Verify(alg Algorithm, secret []byte, token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrBadFormat
	}
	signing := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signing))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return nil, ErrBadSignature
	}
	pb, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrBadFormat
	}
	var c Claims
	if err := json.Unmarshal(pb, &c); err != nil {
		return nil, ErrBadFormat
	}
	if exp, ok := c["exp"]; ok {
		var expSec int64
		switch v := exp.(type) {
		case float64:
			expSec = int64(v)
		case int64:
			expSec = v
		case int:
			expSec = int64(v)
		}
		if time.Now().Unix() > expSec {
			return c, ErrExpired
		}
	}
	return c, nil
}

// Issue 便捷封装：设置 sub/iat/exp 并签发。
func Issue(secret []byte, sub string, ttl time.Duration, extra Claims) (string, error) {
	now := time.Now()
	c := Claims{
		"sub": sub,
		"iat": now.Unix(),
		"exp": now.Add(ttl).Unix(),
	}
	for k, v := range extra {
		c[k] = v
	}
	return Sign(HS256, secret, c)
}
