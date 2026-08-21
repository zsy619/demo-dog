package jwt

// verifier.go:Verifier 主体。
//
// Verifier 维护一个按 kid 索引的 Key 环,当前 key 始终位于索引 0。
// Add/Remove 维护顺序;Verify 解析 JWT 并按 kid 查找签名密钥。

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"sync"
	"time"
)

// Verifier 按 kid 保存一个 key 环,
// 当前 key 始终位于索引 0。
type Verifier struct {
	mu        sync.RWMutex    // 保护 keys / order
	keys      map[string]*Key // kid → Key
	order     []string        // kid 顺序(当前 key 在索引 0)
	clockSkew time.Duration   // 时间偏差容忍
	now       func() time.Time // 时间源(测试可注入)
}

// New 创建一个空的 Verifier。
//
// clockSkew <= 0 视为 30 秒。
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

// Verify 解析并校验令牌,返回 kid。
//
// 校验流程:分割 → base64url 解码 header/body → 用 kid 查 key
// → HMAC 校验签名 → 校验 exp/nbf 时间戳。
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
