// Package cipher 提供 AES-GCM 的高层包装：
// Seal 返回的密文将 nonce 拼接到密文前，Open 时自动剥离。
package cipher

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
)

// ErrKeyLength 在密钥长度不合法时返回。
var ErrKeyLength = errors.New("cipher: 密钥长度不合法")

// ErrShortMessage 在密文过短时返回。
var ErrShortMessage = errors.New("cipher: 密文过短")

// SealAESGCM 用 AES-GCM 加密，nonce 自动生成并拼接到密文前。
func SealAESGCM(key, plaintext, aad []byte) ([]byte, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, ErrKeyLength
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
	sealed := gcm.Seal(nil, nonce, plaintext, aad)
	return append(nonce, sealed...), nil
}

// OpenAESGCM 解密 SealAESGCM 输出。
func OpenAESGCM(key, ciphertext, aad []byte) ([]byte, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, ErrKeyLength
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(ciphertext) < ns {
		return nil, ErrShortMessage
	}
	nonce := ciphertext[:ns]
	body := ciphertext[ns:]
	return gcm.Open(nil, nonce, body, aad)
}

// DeriveKey 用 SHA-256 将口令转换为 32 字节密钥。
func DeriveKey(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

// Box 提供一个简化的命名空间入口。
type Box struct {
	Key []byte
	AAD []byte
}

// Seal 加密为 AES-GCM。
func (b *Box) Seal(p []byte) ([]byte, error) { return SealAESGCM(b.Key, p, b.AAD) }

// Open 解密。
func (b *Box) Open(c []byte) ([]byte, error) { return OpenAESGCM(b.Key, c, b.AAD) }

// RandomKey 生成指定位数的随机 AES 密钥（128/192/256）。
func RandomKey(bits int) ([]byte, error) {
	if bits != 128 && bits != 192 && bits != 256 {
		return nil, errors.New("cipher: bits 必须是 128/192/256")
	}
	k := make([]byte, bits/8)
	if _, err := io.ReadFull(rand.Reader, k); err != nil {
		return nil, err
	}
	return k, nil
}
