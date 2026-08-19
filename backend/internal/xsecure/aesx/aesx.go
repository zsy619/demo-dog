// Package aesx 提供 AES-GCM 加密 / 解密辅助。
package aesx

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
)

// KeyLengthError 表示密钥长度错误。
type KeyLengthError int

func (k KeyLengthError) Error() string {
	return "aesx: 密钥长度必须是 16/24/32 字节"
}

// Encrypt 使用 AES-GCM 加密（nonce 前缀输出）。
func Encrypt(key, plaintext []byte) ([]byte, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, KeyLengthError(len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	out := gcm.Seal(nil, nonce, plaintext, nil)
	return append(nonce, out...), nil
}

// Decrypt 解密 Encrypt 输出。
func Decrypt(key, blob []byte) ([]byte, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, KeyLengthError(len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(blob) < gcm.NonceSize() {
		return nil, errors.New("aesx: 数据太短")
	}
	nonce, ct := blob[:gcm.NonceSize()], blob[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ct, nil)
}

// RandomKey 生成随机 AES 密钥。
func RandomKey(bits int) ([]byte, error) {
	if bits != 128 && bits != 192 && bits != 256 {
		return nil, errors.New("aesx: bits 必须是 128/192/256")
	}
	k := make([]byte, bits/8)
	if _, err := io.ReadFull(rand.Reader, k); err != nil {
		return nil, err
	}
	return k, nil
}
