package jwt

// sign.go:Sign 与 Middleware。
//
// Sign 用给定 Key 签发 HS256 令牌;
// Middleware 校验 Authorization Bearer 并把 claims 注入 context。

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
)

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

// Middleware 返回要求有效 bearer token 的 http.Handler,
// 并将 claims 注入请求上下文。
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
