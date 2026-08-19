// Package cap 提供能力令牌（Capability Token）机制：
// 一个 Token 代表一组权限范围（scopes）和资源（resources），
// 用于细粒度的接口授权。
package cap

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ErrInvalid 在签名验证失败或格式错误时返回。
var ErrInvalid = errors.New("cap: 令牌无效")

// ErrExpired 在令牌过期时返回。
var ErrExpired = errors.New("cap: 令牌过期")

// Token 是能力令牌内容。
type Token struct {
	Subject  string    `json:"sub"`
	Scopes   []string  `json:"scopes"`
	Resource string    `json:"res,omitempty"`
	IssuedAt time.Time `json:"iat"`
	Expires  time.Time `json:"exp"`
}

// HasScope 返回是否拥有 scope。
func (t *Token) HasScope(s string) bool {
	for _, sc := range t.Scopes {
		if sc == s || sc == "*" {
			return true
		}
	}
	return false
}

// HasResource 返回资源是否匹配。
func (t *Token) HasResource(r string) bool {
	if t.Resource == "" || t.Resource == "*" {
		return true
	}
	return t.Resource == r
}

// Issued 创建一个能力令牌。
func Issued(secret []byte, sub, resource string, scopes []string, ttl time.Duration) (string, error) {
	now := time.Now()
	t := Token{
		Subject:  sub,
		Scopes:   scopes,
		Resource: resource,
		IssuedAt: now,
		Expires:  now.Add(ttl),
	}
	body, err := json.Marshal(t)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	sig := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(body) + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// Verify 校验令牌签名与有效性。
func Verify(secret []byte, raw string) (*Token, error) {
	parts := strings.SplitN(raw, ".", 2)
	if len(parts) != 2 {
		return nil, ErrInvalid
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrInvalid
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalid
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return nil, ErrInvalid
	}
	var t Token
	if err := json.Unmarshal(body, &t); err != nil {
		return nil, ErrInvalid
	}
	if !t.Expires.IsZero() && time.Now().After(t.Expires) {
		return nil, ErrExpired
	}
	return &t, nil
}

// Authorize 检查 token 是否满足 scope+resource。
func Authorize(secret []byte, raw, scope, resource string) error {
	t, err := Verify(secret, raw)
	if err != nil {
		return err
	}
	if !t.HasScope(scope) {
		return fmt.Errorf("cap: 缺少 scope %s", scope)
	}
	if !t.HasResource(resource) {
		return fmt.Errorf("cap: 资源不匹配")
	}
	return nil
}

// Secret 是生成秘钥的便捷函数。
func Secret() ([]byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

// Cache 是一个简单的内存能力缓存，避免重复验签。
type Cache struct {
	mu    sync.Mutex
	store map[string]*Token
}

// NewCache 创建一个空 Cache。
func NewCache() *Cache {
	return &Cache{store: make(map[string]*Token)}
}

// Get 取已校验的令牌。
func (c *Cache) Get(raw string) (*Token, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	t, ok := c.store[raw]
	return t, ok
}

// Put 缓存一个已校验的令牌。
func (c *Cache) Put(raw string, t *Token) {
	c.mu.Lock()
	c.store[raw] = t
	c.mu.Unlock()
}

// Clear 清空。
func (c *Cache) Clear() {
	c.mu.Lock()
	c.store = make(map[string]*Token)
	c.mu.Unlock()
}
