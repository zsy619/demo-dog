// Package secretbox 提供基于 AES-GCM 的 key 包装（wrap/unwrap）。
// 用于临时加密敏感数据（如数据库密码、Token）后保存到配置。
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
)

// ErrBadSize 在 wrap key 长度非 16/24/32 时返回。
var ErrBadSize = errors.New("secretbox: wrap key 长度错误")

// ErrOpen 在解包失败时返回。
var ErrOpen = errors.New("secretbox: 解包失败")

// Wrap 使用 wrapKey 加密 plaintext，输出 nonce|ciphertext|tag。
func Wrap(wrapKey, plaintext []byte) ([]byte, error) {
	block, err := cipherFor(wrapKey)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ct := aead.Seal(nil, nonce, plaintext, nil)
	out := make([]byte, 0, len(nonce)+len(ct))
	out = append(out, nonce...)
	out = append(out, ct...)
	return out, nil
}

// Unwrap 解包由 Wrap 加密的密文。
func Unwrap(wrapKey, packed []byte) ([]byte, error) {
	block, err := cipherFor(wrapKey)
	if err != nil {
		return nil, err
	}
	if len(packed) < 12 {
		return nil, ErrOpen
	}
	nonce := packed[:12]
	ct := packed[12:]
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	pt, err := aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, ErrOpen
	}
	return pt, nil
}

// RandomKey 返回 32 字节随机密钥。
func RandomKey() ([]byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

func cipherFor(k []byte) (cipher.Block, error) {
	if l := len(k); l != 16 && l != 24 && l != 32 {
		return nil, ErrBadSize
	}
	return aes.NewCipher(k)
}
