// Package encx 提供 base64 / base32 / hex 编码的便捷封装。
package encx

import (
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
)

// Base64Enc 是 base64.StdEncoding 封装。
func Base64Enc(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

// Base64Dec 解码 base64。
func Base64Dec(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// Base64URLEnc 是 URL 安全的 base64。
func Base64URLEnc(b []byte) string {
	return base64.URLEncoding.EncodeToString(b)
}

// Base64URLDec 解码 URL base64。
func Base64URLDec(s string) ([]byte, error) {
	return base64.URLEncoding.DecodeString(s)
}

// Base32Enc 是 base32.StdEncoding。
func Base32Enc(b []byte) string {
	return base32.StdEncoding.EncodeToString(b)
}

// Base32Dec 解码 base32。
func Base32Dec(s string) ([]byte, error) {
	return base32.StdEncoding.DecodeString(s)
}

// HexEnc 是 hex 编码。
func HexEnc(b []byte) string {
	return hex.EncodeToString(b)
}

// HexDec 解码 hex。
func HexDec(s string) ([]byte, error) {
	return hex.DecodeString(s)
}
