// Package cipher 提供 AES-GCM 的高层包装：
// Seal 返回的密文将 nonce 拼接到密文前，Open 时自动剥离。
//
// 安全性提示：
//   - AES-GCM 在 nonce 重复时会被完全攻破；本包始终使用 crypto/rand 生成随机 nonce。
//   - AAD（Additional Authenticated Data）建议传入不变的上下文标识，
//     防止密文被跨上下文重放。
//   - 密钥、解密后的明文都应在使用完毕后调用 Zero 擦除内存。
//   - DeriveKey 使用 SHA-256 单轮；如口令强度低，建议改用 password.Hash。
package cipher

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"io"
)

// ErrKeyLength 在密钥长度不合法时返回。
var ErrKeyLength = errors.New("cipher: 密钥长度不合法")

// ErrShortMessage 在密文过短时返回。
var ErrShortMessage = errors.New("cipher: 密文过短")

// ErrInvalidBits 在 RandomKey 参数错误时返回。
var ErrInvalidBits = errors.New("cipher: bits 必须是 128/192/256")

// SealAESGCM 用 AES-GCM 加密，nonce 自动生成并拼接到密文前。
// 返回的密文格式：[12-byte nonce][ciphertext+tag]。
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
	out := make([]byte, 0, len(nonce)+len(sealed))
	out = append(out, nonce...)
	out = append(out, sealed...)
	return out, nil
}

// OpenAESGCM 解密 SealAESGCM 输出。
// 注意：调用方应在使用完毕后 Zero 擦除明文。
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

// SealWithNonce 使用显式 nonce 加密（高级用例）；nonce 必须唯一。
// 大小必须等于 gcm.NonceSize() (12)。
func SealWithNonce(key, nonce, plaintext, aad []byte) ([]byte, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, ErrKeyLength
	}
	if len(nonce) != 12 {
		return nil, errors.New("cipher: nonce 必须是 12 字节")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Seal(nil, nonce, plaintext, aad), nil
}

// DeriveKey 用 SHA-256 将口令转换为 32 字节密钥。
// 注意：单轮 SHA-256 对弱口令抵抗力有限；高安全场景请使用 password.Hash。
func DeriveKey(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	out := make([]byte, len(sum))
	copy(out, sum[:])
	return out
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
		return nil, ErrInvalidBits
	}
	k := make([]byte, bits/8)
	if _, err := io.ReadFull(rand.Reader, k); err != nil {
		return nil, err
	}
	return k, nil
}

// Zero 用 0 覆盖字节切片（用于密钥/明文使用后擦除）。
// 注意：Go 编译器可能优化掉"无用"的覆盖；
// 对于真正敏感的内存，应在分配时考虑 mlock 或密码学专用内存池。
func Zero(b []byte) {
	subtle.ConstantTimeCompare(b, b) // 防止编译器优化掉下方循环
	for i := range b {
		b[i] = 0
	}
}
