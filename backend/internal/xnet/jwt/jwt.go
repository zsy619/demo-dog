// Package jwt JWT 签发与校验：支持多种签名算法。
package jwt

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Algorithm 表示签名算法。
type Algorithm string

const (
	HS256 Algorithm = "HS256"
)

// Key 保存一次轮换项的密钥与 kid。
type Key struct {
	KID     string
	Secret  []byte
	Alg     Algorithm
	Created time.Time
}

// Verifier 持有按 kid 索引的密钥环，
// 当前密钥始终位于索引 0。
type Verifier struct {
	mu        sync.RWMutex
	keys      map[string]*Key
	order     []string
	clockSkew time.Duration
	now       func() time.Time
}

// New 创建一个空的 Verifier。
func New(clockSkew time.Duration) *Verifier {
	if clockSkew <= 0 {
		clockSkew = 30 * time.Second
	}
	return &Verifier{
		keys:      make(map[string]*Key),
		clockSkew: clockSkew,
		now:       time.Now,
	}
}

// WithTime 覆盖测试所用的时间源。
func (v *Verifier) WithTime(now func() time.Time) *Verifier {
	v.now = now
	return v
}

// Add 引入一个新密钥。新密钥会放到顺序的最前面。
func (v *Verifier) Add(k *Key) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.keys[k.KID] = k
	v.order = append([]string{k.KID}, v.order...)
}

// Remove 按 kid 删除一个密钥。
func (v *Verifier) Remove(kid string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.keys, kid)
	out := v.order[:0]
	for _, k := range v.order {
		if k != kid {
			out = append(out, k)
		}
	}
	v.order = out
}

// ErrNoKey 在没有密钥匹配给定 kid 时返回。
var ErrNoKey = errors.New("no key")

// ErrBadToken 在令牌格式错误时返回。
var ErrBadToken = errors.New("bad token")

// ErrExpired 在令牌超过有效期时返回。
var ErrExpired = errors.New("expired")

// Verify 解析并校验令牌，返回 kid。
func (v *Verifier) Verify(token string) (map[string]any, string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, "", ErrBadToken
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, "", ErrBadToken
	}
	var header map[string]any
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, "", ErrBadToken
	}
	kid, _ := header["kid"].(string)
	v.mu.RLock()
	k, ok := v.keys[kid]
	v.mu.RUnlock()
	if !ok {
		return nil, "", ErrNoKey
	}
	mac := hmac.New(sha256.New, k.Secret)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	if macStr := base64.RawURLEncoding.EncodeToString(mac.Sum(nil)); macStr != parts[2] {
		return nil, "", ErrBadToken
	}
	bodyBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, "", ErrBadToken
	}
	var claims map[string]any
	if err := json.Unmarshal(bodyBytes, &claims); err != nil {
		return nil, "", ErrBadToken
	}
	if exp, ok := claims["exp"].(float64); ok {
		if v.now().Unix() > int64(exp) {
			return claims, kid, ErrExpired
		}
	}
	if nbf, ok := claims["nbf"].(float64); ok {
		if v.now().Unix() < int64(nbf) {
			return claims, kid, ErrBadToken
		}
	}
	return claims, kid, nil
}

type ctxKey int

const (
	claimsKey ctxKey = iota
	kidKey
)

type claimsCtx struct {
	ctx    context.Context
	claims map[string]any
	kid    string
}

func (c *claimsCtx) Value(key any) any {
	switch key {
	case claimsKey:
		return c.claims
	case kidKey:
		return c.kid
	}
	return c.ctx.Value(key)
}

func (c *claimsCtx) Deadline() (time.Time, bool)       { return c.ctx.Deadline() }
func (c *claimsCtx) Done() <-chan struct{}             { return c.ctx.Done() }
func (c *claimsCtx) Err() error                        { return c.ctx.Err() }

// WithClaims 将 claims 与 kid 包装到 context.Context 中。
func WithClaims(ctx context.Context, claims map[string]any, kid string) context.Context {
	return &claimsCtx{ctx: ctx, claims: claims, kid: kid}
}

// FromContext 返回由中间件注入到上下文中的 claims 映射
// 与 kid。
func FromContext(ctx context.Context) (map[string]any, string, bool) {
	c, ok := ctx.Value(claimsKey).(map[string]any)
	if !ok {
		return nil, "", false
	}
	kid, _ := ctx.Value(kidKey).(string)
	return c, kid, true
}

// Sign 使用给定密钥为指定 claims 签发一个
// HS256 令牌。
func Sign(claims map[string]any, k *Key) (string, error) {
	header := map[string]any{"alg": "HS256", "typ": "JWT", "kid": k.KID}
	hb, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	cb, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	hp := base64.RawURLEncoding.EncodeToString(hb)
	cp := base64.RawURLEncoding.EncodeToString(cb)
	mac := hmac.New(sha256.New, k.Secret)
	mac.Write([]byte(hp + "." + cp))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return hp + "." + cp + "." + sig, nil
}

// Middleware 返回一个 http.Handler，要求传入合法的
// bearer 令牌，并将 claims 注入到请求上下文中。
func (v *Verifier) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "missing bearer", http.StatusUnauthorized)
			return
		}
		tok := strings.TrimPrefix(auth, "Bearer ")
		claims, kid, err := v.Verify(tok)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := WithClaims(r.Context(), claims, kid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
