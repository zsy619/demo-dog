// Package sig 签名校验：通用请求签名工具。
package sig

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"sync"
)

// KeyPair 是一个 Ed25519 密钥对。
type KeyPair struct {
	Public  ed25519.PublicKey
	Private ed25519.PrivateKey
}

// Generate 创建一个新的 Ed25519 密钥对。
func Generate() (KeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return KeyPair{}, err
	}
	return KeyPair{Public: pub, Private: priv}, nil
}

// Sign 返回 msg 的签名。
func (k KeyPair) Sign(msg []byte) []byte {
	return ed25519.Sign(k.Private, msg)
}

// Verify 检查 msg 的签名。
func (k KeyPair) Verify(msg, sig []byte) bool {
	return ed25519.Verify(k.Public, msg, sig)
}

// Verifier 持有按 ID 索引的公钥注册表。
type Verifier struct {
	mu    sync.RWMutex
	keys  map[string]ed25519.PublicKey
}

// NewVerifier 创建一个空的 Verifier。
func NewVerifier() *Verifier {
	return &Verifier{keys: make(map[string]ed25519.PublicKey)}
}

// Add 在 id 下注册一个公钥。
func (v *Verifier) Add(id string, pub ed25519.PublicKey) {
	v.mu.Lock()
	v.keys[id] = pub
	v.mu.Unlock()
}

// Remove 移除一个密钥。
func (v *Verifier) Remove(id string) {
	v.mu.Lock()
	delete(v.keys, id)
	v.mu.Unlock()
}

// ErrUnknownKey 在密钥 ID 未注册时返回。
var ErrUnknownKey = errors.New("unknown key")

// ErrBadSignature 在签名无效时返回。
var ErrBadSignature = errors.New("bad signature")

// VerifyMessage looks up the key for id and verifies msg
// against sig.
func (v *Verifier) VerifyMessage(id string, msg, sig []byte) error {
	v.mu.RLock()
	pub, ok := v.keys[id]
	v.mu.RUnlock()
	if !ok {
		return ErrUnknownKey
	}
	if !ed25519.Verify(pub, msg, sig) {
		return ErrBadSignature
	}
	return nil
}

// IDs 返回已注册的密钥 ID 列表。
func (v *Verifier) IDs() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	out := make([]string, 0, len(v.keys))
	for id := range v.keys {
		out = append(out, id)
	}
	return out
}
