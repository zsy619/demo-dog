package auth

// token.go：密码学 token 生成与哈希。
//
// 这些是 AdminStore 内部使用的辅助函数。

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// GenerateToken 返回 32 字节的密码学随机 hex token。
//
// 调用方负责只将结果向用户展示一次（创建时）。
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// hashToken 返回 token 的小写 hex sha256。
func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}
