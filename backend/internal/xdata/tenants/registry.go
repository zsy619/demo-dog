// Package tenants 实现一个轻量的进程内租户注册表。每个
// 租户 has a unique ID, a display name, an optional description, and
// a list of API keys that belong to it. The registry is the source of
// truth for 租户 isolation: when a handler resolves a request it
// consults the registry to decide which 租户 the request belongs to.
//
// In Round 23 the registry lives in memory and is seeded from the CLI
// flag `-tenants`. A future round can swap it for a SQLite-backed
// implementation without changing the public API.
package tenants

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
)

var ErrNotFound = errors.New("tenant not found")

// Tenant 是组织的线上描述。
type Tenant struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	Active      bool      `json:"active"`
}

// Key 是由一个租户拥有的 API key。我们不持久化 secret；
// caller is responsible for handing the plaintext to the operator over
// a secure 通道.
type Key struct {
	TenantID  string    `json:"tenant_id"`
	Label     string    `json:"label"`
	Plaintext string    `json:"plaintext"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// Registry 是内存中的租户 + key 存储。
type Registry struct {
	mu       sync.RWMutex
	tenants  map[string]*Tenant
	keys     map[string]string // plaintext key -> tenant ID
}

func New() *Registry {
	return &Registry{
		tenants: make(map[string]*Tenant),
		keys:    make(map[string]string),
	}
}

// CreateTenant 注册一个新租户。ID 被规范化为小写
// slug; 空的 ID 返回 an error.
func (r *Registry) CreateTenant(id, name, description string) (*Tenant, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return nil, errors.New("tenant id is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tenants[id]; exists {
		return nil, errors.New("tenant id already in use")
	}
	t := &Tenant{
		ID:          id,
		Name:        name,
		Description: description,
		CreatedAt:   time.Now().UTC(),
		Active:      true,
	}
	r.tenants[id] = t
	return t, nil
}

// Get 按 id 返回一个租户。
func (r *Registry) Get(id string) (*Tenant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tenants[id]
	if !ok {
		return nil, ErrNotFound
	}
	out := *t
	return &out, nil
}

// List 按插入顺序（按 created_at）返回所有租户。
func (r *Registry) List() []Tenant {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Tenant, 0, len(r.tenants))
	for _, t := range r.tenants {
		out = append(out, *t)
	}
	return out
}

// MintKey 为给定租户 + 角色生成新 API key。
// plaintext is returned exactly once so the caller can hand it to the
// human operator.
func (r *Registry) MintKey(tenantID, label, role string) (*Key, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tenants[tenantID]; !ok {
		return nil, ErrNotFound
	}
	plaintext := randomKey()
	r.keys[plaintext] = tenantID
	return &Key{
		TenantID:  tenantID,
		Label:     label,
		Plaintext: plaintext,
		Role:      role,
		CreatedAt: time.Now().UTC(),
	}, nil
}

// LookupTenant 返回拥有 key 的租户，若无则返回空
// key is not a tenant-bound key. 由...使用 the auth 中间件 to stamp
// X-Dog-Tenant on the request.
func (r *Registry) LookupTenant(plaintext string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.keys[plaintext]
}

// SnapshotKeyMap 返回每个 (明文 -> 租户 id) 对的副本
// so the auth layer can build its registry without losing information.
func (r *Registry) SnapshotKeyMap() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]string, len(r.keys))
	for k, v := range r.keys {
		out[k] = v
	}
	return out
}

func randomKey() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return "dog_" + hex.EncodeToString(b)
}
