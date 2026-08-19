// Package sealedx 提供对称密钥封装的 Seal/Open：
// 把 (key, nonce) + 密文 打包为单一字节切片。
package sealedx

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
)

const nonceSize = 12

// Seal 加密 plaintext 并返回 [nonce|ciphertext]。
func Seal(key, plaintext []byte) ([]byte, error) {
	if len(key) != 16 && len(key) != 32 {
		return nil, errors.New("sealedx: key 长度必须为 16 或 32")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	ct := gcm.Seal(nil, nonce, plaintext, nil)
	out := make([]byte, 0, nonceSize+len(ct))
	out = append(out, nonce...)
	out = append(out, ct...)
	return out, nil
}

// Open 解密 [nonce|ciphertext]。
func Open(key, blob []byte) ([]byte, error) {
	if len(blob) < nonceSize {
		return nil, errors.New("sealedx: 数据过短")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, blob[:nonceSize], blob[nonceSize:], nil)
}
