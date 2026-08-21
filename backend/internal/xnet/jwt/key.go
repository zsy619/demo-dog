// Package jwt JWT 签发与校验:支持多种签名算法。
//
// 文件职责拆分:
//   - key.go      Algorithm 常量 + Key 类型 + 错误变量
//   - verifier.go Verifier 主体与 Verify
//   - context.go  claimsCtx + WithClaims + FromContext
//   - sign.go     Sign + Middleware
package jwt

import (
	"errors"
	"time"
)

// Algorithm 表示签名算法。
type Algorithm string

const (
	HS256 Algorithm = "HS256" // HMAC-SHA256
)

// Key 保存一次轮换项的密钥与 kid。
type Key struct {
	KID     string    // 密钥 ID
	Secret  []byte    // 密钥字节
	Alg     Algorithm // 签名算法
	Created time.Time // 创建时间
}

// ErrNoKey 在没有密钥匹配给定 kid 时返回。
var ErrNoKey = errors.New("no key")

// ErrBadToken 在令牌格式错误时返回。
var ErrBadToken = errors.New("bad token")

// ErrExpired 在令牌超过有效期时返回。
var ErrExpired = errors.New("expired")
