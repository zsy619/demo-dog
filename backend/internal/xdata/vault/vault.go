// Package vault 凭证保管:加密存储 API key / 密码等敏感配置,支持生命周期管理。
//
// 文件职责拆分:
//   - vault.go  Vault 主体 + Put/Get/Delete/Names/Audit
//   - entry.go  Entry + AuditEntry 类型定义
//   - crypto.go 加密/解密 + EncodeAudit
package vault

import (
	"errors"
	"sync"
	"time"
)

// Vault 是加密秘密存储。
//
// 每个值用 per-tenant key 加密,通过 SHA256(master || tenant) 派生;
// vault 自身永不存储明文。
type Vault struct {
	mu       sync.RWMutex             // 保护 entries / audit
	master   []byte                   // 主密钥(复制保存)
	entries  map[string]map[string]*Entry // tenant → name → entry
	audit    []AuditEntry             // 审计日志(滚动)
	auditCap int                      // 审计日志容量上限
	now      func() time.Time         // 时钟源(便于测试)
}

// NewVault 创建一个由主密钥派生的 Vault。
//
// auditCap <= 0 默认 1024;master 至少 16 字节,否则 panic。
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

// WithTime 替换时钟源(供测试)。
func (v *Vault) WithTime(now func() time.Time) *Vault {
	v.now = now
	return v
}

// Put 在租户密钥下加密 plaintext 并存储。
//
// 返回分配的版本号。
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

// Get 解密 tenant + name 对应的秘密。
//
// 返回 (明文, 是否存在, 错误)。
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

// Delete 移除一个秘密。
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

// Names 列出某租户下存储的秘密名。
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

// Audit 返回审计日志的副本。
func (v *Vault) Audit() []AuditEntry {
	v.mu.RLock()
	defer v.mu.RUnlock()
	out := make([]AuditEntry, len(v.audit))
	copy(out, v.audit)
	return out
}

// recordAudit 追加一条审计记录;超出容量则丢弃最旧。
func (v *Vault) recordAudit(e AuditEntry) {
	v.mu.Lock()
	if len(v.audit) >= v.auditCap {
		v.audit = v.audit[1:]
	}
	v.audit = append(v.audit, e)
	v.mu.Unlock()
}
