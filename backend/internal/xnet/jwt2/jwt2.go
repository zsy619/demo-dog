// Package jwt2 提供从 HTTP 请求头提取 Bearer Token 与常见 JWKS 处理。
package jwt2

import (
	"encoding/json"
	"errors"
	"strings"
)

// ErrNoToken 在缺少 Bearer Token 时返回。
var ErrNoToken = errors.New("jwt2: 缺少 token")

// ExtractBearer 从 Authorization 头中提取 Bearer Token。
func ExtractBearer(auth string) (string, error) {
	const p = "Bearer "
	if !strings.HasPrefix(auth, p) {
		return "", ErrNoToken
	}
	tok := strings.TrimSpace(auth[len(p):])
	if tok == "" {
		return "", ErrNoToken
	}
	return tok, nil
}

// JWKS 是一个 JWKS 文档的简化表示。
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWK 是单个 JSON Web Key。
type JWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n,omitempty"`
	E   string `json:"e,omitempty"`
	X   string `json:"x,omitempty"`
	Y   string `json:"y,omitempty"`
	Crv string `json:"crv,omitempty"`
	K   string `json:"k,omitempty"`
}

// ParseJWKS 解析 JWKS JSON。
func ParseJWKS(b []byte) (JWKS, error) {
	var j JWKS
	if err := json.Unmarshal(b, &j); err != nil {
		return j, err
	}
	return j, nil
}

// FindKID 在 JWKS 中查找匹配的 kid。
func (j JWKS) FindKID(kid string) (JWK, bool) {
	for _, k := range j.Keys {
		if k.Kid == kid {
			return k, true
		}
	}
	return JWK{}, false
}
