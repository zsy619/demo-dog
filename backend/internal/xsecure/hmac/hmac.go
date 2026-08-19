// Package hmac 封装 HMAC-SHA256 / HMAC-SHA1 / HMAC-SHA512 签名。
package hmac

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
)

// SHA256 返回 hex 编码的 HMAC-SHA256。
func SHA256(key, msg []byte) string {
	m := hmac.New(sha256.New, key)
	m.Write(msg)
	return hex.EncodeToString(m.Sum(nil))
}

// SHA1 返回 hex 编码的 HMAC-SHA1。
func SHA1(key, msg []byte) string {
	m := hmac.New(sha1.New, key)
	m.Write(msg)
	return hex.EncodeToString(m.Sum(nil))
}

// SHA512 返回 hex 编码的 HMAC-SHA512。
func SHA512(key, msg []byte) string {
	m := hmac.New(sha512.New, key)
	m.Write(msg)
	return hex.EncodeToString(m.Sum(nil))
}

// SHA256B64 返回 base64 编码的 HMAC-SHA256。
func SHA256B64(key, msg []byte) string {
	m := hmac.New(sha256.New, key)
	m.Write(msg)
	return base64.StdEncoding.EncodeToString(m.Sum(nil))
}

// VerifySHA256 在常数时间内比较签名。
func VerifySHA256(key, msg []byte, sigHex string) bool {
	expected := SHA256(key, msg)
	return hmac.Equal([]byte(expected), []byte(sigHex))
}
