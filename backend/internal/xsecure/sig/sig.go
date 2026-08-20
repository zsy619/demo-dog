package sig

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"sync"
)

// KeyPair is an Ed25519 key pair.
type KeyPair struct {
	Public  ed25519.PublicKey
	Private ed25519.PrivateKey
}

// Generate creates a new Ed25519 key pair.
func Generate() (KeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return KeyPair{}, err
	}
	return KeyPair{Public: pub, Private: priv}, nil
}

// Sign returns the signature for msg.
func (k KeyPair) Sign(msg []byte) []byte {
	return ed25519.Sign(k.Private, msg)
}

// Verify checks the signature for msg.
func (k KeyPair) Verify(msg, sig []byte) bool {
	return ed25519.Verify(k.Public, msg, sig)
}

// Verifier holds a registry of public keys keyed by ID.
type Verifier struct {
	mu    sync.RWMutex
	keys  map[string]ed25519.PublicKey
}

// NewVerifier creates an empty Verifier.
func NewVerifier() *Verifier {
	return &Verifier{keys: make(map[string]ed25519.PublicKey)}
}

// Add registers a public key under id.
func (v *Verifier) Add(id string, pub ed25519.PublicKey) {
	v.mu.Lock()
	v.keys[id] = pub
	v.mu.Unlock()
}

// Remove drops a key.
func (v *Verifier) Remove(id string) {
	v.mu.Lock()
	delete(v.keys, id)
	v.mu.Unlock()
}

// ErrUnknownKey is returned when the key ID is not registered.
var ErrUnknownKey = errors.New("unknown key")

// ErrBadSignature is returned when the signature is invalid.
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

// IDs returns the registered key IDs.
func (v *Verifier) IDs() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	out := make([]string, 0, len(v.keys))
	for id := range v.keys {
		out = append(out, id)
	}
	return out
}
