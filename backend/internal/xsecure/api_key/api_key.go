// Package api_key 提供 API Key 签发与校验：
// 形如 ak_xxx.yyyy，yyy 是 base64 编码的 HMAC 签名。
package api_key

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
)

// ErrBadFormat 在 key 格式错误时返回。
var ErrBadFormat = errors.New("api_key: 格式错误")

// ErrBadSignature 在签名不匹配时返回。
var ErrBadSignature = errors.New("api_key: 签名错误")

// Manager 持有 secret 并校验 key。
type Manager struct {
	mu     sync.RWMutex
	secret []byte
}

// New 创建一个 Manager。
func New(secret string) *Manager {
	return &Manager{secret: []byte(secret)}
}

// Issue 返回 ak_<id>.<sig> 形式的字符串。
func (m *Manager) Issue(id string) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(id))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return "ak_" + id + "." + sig
}

// Verify 校验 key 并返回 id。
func (m *Manager) Verify(key string) (string, error) {
	if !strings.HasPrefix(key, "ak_") {
		return "", ErrBadFormat
	}
	parts := strings.SplitN(key[3:], ".", 2)
	if len(parts) != 2 {
		return "", ErrBadFormat
	}
	id, sig := parts[0], parts[1]
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(id))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return "", ErrBadSignature
	}
	return id, nil
}

// Hash 返回 key 的 SHA256 摘要（hex），用于持久化存储。
func Hash(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// StripPrefix 去除 ak_ 前缀。
func StripPrefix(key string) string {
	return strings.TrimPrefix(key, "ak_")
}
