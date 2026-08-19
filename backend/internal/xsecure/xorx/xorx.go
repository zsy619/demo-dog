// Package xorx 提供一个简化的 XOR 流密码。
// 适用于轻量场景；不应用于高安全性需求。
package xorx

// Cipher 是一个基于字节 key 的 XOR 密码。
type Cipher struct {
	key []byte
}

// New 创建一个 XOR 密码。
func New(key []byte) *Cipher {
	cp := make([]byte, len(key))
	copy(cp, key)
	return &Cipher{key: cp}
}

// Encrypt 加密 plaintext（XOR 为对称）。
func (c *Cipher) Encrypt(plaintext []byte) []byte {
	return c.xor(plaintext)
}

// Decrypt 解密密文。
func (c *Cipher) Decrypt(ciphertext []byte) []byte {
	return c.xor(ciphertext)
}

func (c *Cipher) xor(in []byte) []byte {
	out := make([]byte, len(in))
	for i, b := range in {
		out[i] = b ^ c.key[i%len(c.key)]
	}
	return out
}

// Bytes 直接加密 src 并返回 dst。
func Bytes(key, src []byte) []byte {
	return New(key).Encrypt(src)
}
