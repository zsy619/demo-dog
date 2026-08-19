// Package tokenx 提供简单 token 生成与校验：
// base64url(payload).sig, 签名使用 HMAC-SHA256。
package tokenx

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

// ErrBadFormat 表示 token 格式错误。
var ErrBadFormat = errors.New("tokenx: 格式错误")

// ErrBadSig 表示签名错误。
var ErrBadSig = errors.New("tokenx: 签名错误")

// Payload 是 token 的负载。
type Payload struct {
	Subject   string         `json:"sub,omitempty"`
	ExpiresAt int64          `json:"exp,omitempty"`
	Extra     map[string]any `json:"ext,omitempty"`
}

// Sign 用 secret 签发 token。
func Sign(p Payload, secret []byte) (string, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	enc := base64.RawURLEncoding.EncodeToString(b)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(enc))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return enc + "." + sig, nil
}

// Verify 校验 token 合法性，包括有效期。
func Verify(token string, secret []byte, now int64) (Payload, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return Payload{}, ErrBadFormat
	}
	enc, sig := parts[0], parts[1]
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(enc))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(sig)) {
		return Payload{}, ErrBadSig
	}
	b, err := base64.RawURLEncoding.DecodeString(enc)
	if err != nil {
		return Payload{}, ErrBadFormat
	}
	var p Payload
	if err := json.Unmarshal(b, &p); err != nil {
		return Payload{}, ErrBadFormat
	}
	if p.ExpiresAt > 0 && now > p.ExpiresAt {
		return p, errors.New("tokenx: 已过期")
	}
	return p, nil
}

// RandomSecret 返回 32 字节随机密钥。
func RandomSecret() ([]byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}
