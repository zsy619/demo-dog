package vault

// crypto.go:加密/解密 + EncodeAudit。

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"time"
)

// tenantKey 通过 SHA256(master || tenant) 派生 per-tenant 32 字节密钥。
func (v *Vault) tenantKey(tenant string) ([]byte, error) {
	h := sha256.New()
	h.Write(v.master)
	h.Write([]byte(tenant))
	return h.Sum(nil), nil
}

// encrypt 用 AES-GCM 加密 plaintext,返回 (密文, nonce, error)。
func encrypt(key []byte, plaintext string) ([]byte, []byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	ct := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	return ct, nonce, nil
}

// decrypt 用 AES-GCM 解密。
func decrypt(key, ct, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, err
	}
	return pt, nil
}

// EncodeAudit 把一条审计条目编码为 base64 JSON 用于导出。
func EncodeAudit(e AuditEntry) string {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(`{"tenant":%q,"name":%q,"actor":%q,"action":%q,"at":%q,"found":%v}`,
		e.Tenant, e.Name, e.Actor, e.Action, e.At.Format(time.RFC3339Nano), e.Found)))
}
