package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// Entry is one stored secret.
type Entry struct {
	Tenant     string
	Name       string
	Ciphertext []byte
	Nonce      []byte
	Version    int
	CreatedAt  time.Time
	UpdatedAt  time.Time
	CreatedBy  string
}

// AuditEntry records every Get/Put/Delete.
type AuditEntry struct {
	Tenant   string
	Name     string
	Actor    string
	Action   string
	At       time.Time
	Found    bool
}

// Vault is the encrypted secrets store. Each value is
// encrypted with a per-tenant key derived from a master key
// via HKDF-style SHA256(master || tenant). The vault itself
// never stores plaintext.
type Vault struct {
	mu        sync.RWMutex
	master    []byte
	entries   map[string]map[string]*Entry // tenant -> name -> entry
	audit     []AuditEntry
	auditCap  int
	now       func() time.Time
}

// NewVault creates a Vault keyed by the given master secret.
func NewVault(master []byte, auditCap int) *Vault {
	if auditCap <= 0 {
		auditCap = 1024
	}
	if len(master) < 16 {
		panic("vault: master key must be >= 16 bytes")
	}
	return &Vault{
		master:   append([]byte{}, master...),
		entries:  make(map[string]map[string]*Entry),
		auditCap: auditCap,
		now:      time.Now,
	}
}

// WithTime overrides the time source for tests.
func (v *Vault) WithTime(now func() time.Time) *Vault {
	v.now = now
	return v
}

// Put encrypts plaintext under the tenant key and stores it.
// Returns the version number assigned.
func (v *Vault) Put(tenant, name, plaintext, actor string) (int, error) {
	if tenant == "" || name == "" {
		return 0, errors.New("tenant and name required")
	}
	key, err := v.tenantKey(tenant)
	if err != nil {
		return 0, err
	}
	ct, nonce, err := encrypt(key, plaintext)
	if err != nil {
		return 0, err
	}
	v.mu.Lock()
	if _, ok := v.entries[tenant]; !ok {
		v.entries[tenant] = make(map[string]*Entry)
	}
	existing := v.entries[tenant][name]
	now := v.now()
	if existing == nil {
		existing = &Entry{
			Tenant: tenant, Name: name,
			Version: 0, CreatedAt: now, CreatedBy: actor,
		}
		v.entries[tenant][name] = existing
	}
	existing.Ciphertext = ct
	existing.Nonce = nonce
	existing.Version++
	existing.UpdatedAt = now
	ver := existing.Version
	v.mu.Unlock()
	v.recordAudit(AuditEntry{Tenant: tenant, Name: name, Actor: actor, Action: "put", At: now, Found: true})
	return ver, nil
}

// Get decrypts the secret for tenant + name.
func (v *Vault) Get(tenant, name, actor string) (string, bool, error) {
	v.mu.RLock()
	e, ok := v.entries[tenant][name]
	v.mu.RUnlock()
	if !ok {
		v.recordAudit(AuditEntry{Tenant: tenant, Name: name, Actor: actor, Action: "get", At: v.now(), Found: false})
		return "", false, nil
	}
	key, err := v.tenantKey(tenant)
	if err != nil {
		return "", false, err
	}
	pt, err := decrypt(key, e.Ciphertext, e.Nonce)
	if err != nil {
		return "", false, err
	}
	v.recordAudit(AuditEntry{Tenant: tenant, Name: name, Actor: actor, Action: "get", At: v.now(), Found: true})
	return string(pt), true, nil
}

// Delete removes the secret.
func (v *Vault) Delete(tenant, name, actor string) error {
	v.mu.Lock()
	found := true
	if _, ok := v.entries[tenant]; !ok {
		found = false
	} else {
		if _, ok := v.entries[tenant][name]; !ok {
			found = false
		} else {
			delete(v.entries[tenant], name)
		}
	}
	v.mu.Unlock()
	v.recordAudit(AuditEntry{Tenant: tenant, Name: name, Actor: actor, Action: "delete", At: v.now(), Found: found})
	return nil
}

// Names lists the names stored under a tenant.
func (v *Vault) Names(tenant string) []string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	t := v.entries[tenant]
	if len(t) == 0 {
		return nil
	}
	out := make([]string, 0, len(t))
	for n := range t {
		out = append(out, n)
	}
	return out
}

// Audit returns a copy of the audit log.
func (v *Vault) Audit() []AuditEntry {
	v.mu.RLock()
	defer v.mu.RUnlock()
	out := make([]AuditEntry, len(v.audit))
	copy(out, v.audit)
	return out
}

func (v *Vault) recordAudit(e AuditEntry) {
	v.mu.Lock()
	if len(v.audit) >= v.auditCap {
		v.audit = v.audit[1:]
	}
	v.audit = append(v.audit, e)
	v.mu.Unlock()
}

// tenantKey derives a per-tenant 32-byte key via SHA256(master || tenant).
func (v *Vault) tenantKey(tenant string) ([]byte, error) {
	h := sha256.New()
	h.Write(v.master)
	h.Write([]byte(tenant))
	return h.Sum(nil), nil
}

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

// EncodeAudit encodes one audit entry as base64 JSON for export.
func EncodeAudit(e AuditEntry) string {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(`{"tenant":%q,"name":%q,"actor":%q,"action":%q,"at":%q,"found":%v}`,
		e.Tenant, e.Name, e.Actor, e.Action, e.At.Format(time.RFC3339Nano), e.Found)))
}
