// Package ed25519x 包装 Ed25519 签名/校验 + 私钥公钥生成。
package ed25519x

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
)

// KeyPair 是 Ed25519 密钥对。
type KeyPair struct {
	Priv []byte
	Pub  []byte
}

// Generate 生成新的 Ed25519 密钥对。
func Generate() (KeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return KeyPair{}, err
	}
	return KeyPair{Priv: priv, Pub: pub}, nil
}

// Sign 用私钥对 msg 签名。
func Sign(priv []byte, msg []byte) ([]byte, error) {
	if len(priv) != ed25519.PrivateKeySize {
		return nil, errors.New("ed25519x: 私钥长度错误")
	}
	return ed25519.Sign(ed25519.PrivateKey(priv), msg), nil
}

// Verify 用公钥校验签名。
func Verify(pub, msg, sig []byte) (bool, error) {
	if len(pub) != ed25519.PublicKeySize {
		return false, errors.New("ed25519x: 公钥长度错误")
	}
	return ed25519.Verify(ed25519.PublicKey(pub), msg, sig), nil
}

// SignString 字符串便捷签名。
func SignString(priv []byte, s string) ([]byte, error) {
	return Sign(priv, []byte(s))
}

// VerifyString 字符串便捷校验。
func VerifyString(pub, sig []byte, s string) (bool, error) {
	return Verify(pub, []byte(s), sig)
}
