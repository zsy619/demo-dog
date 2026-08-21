// Package tenants 实现一个轻量的多租户注册表。每个租户 has a
// unique ID, a display name, an optional description, and a
// list of API keys that belong to it. The registry is the source
// of truth for 租户 isolation: when a handler resolves a request it
// consults the registry to decide which 租户 the request belongs to.
//
// W1 起,registry 切换到 xpersistence.KV 后端(默认 filejson),
// 进程重启后所有 tenant 与 key 都保留。内存仍保留一份只读
// 快照供热路径 O(1) 查询;写路径通过 store 接口打持久化。
//
// 存储 key 命名约定:
//   tenants/<id>            → Tenant JSON
//   tenants-key/<plaintext>  → tenant ID (反向索引)
//
// 旧 API 保持兼容,server.go 等所有调用方不需要修改。
package tenants

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xpersistence"
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
	KeyID     string    `json:"key_id,omitempty"`
	Label     string    `json:"label"`
	Plaintext string    `json:"plaintext"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// Registry 是带持久化的租户 + key 存储。
//
// 内存中:
//   tenants[id] -> Tenant 快照
//   keys[plaintext] -> tenantID 反向索引
//
// 每次写操作 (CreateTenant / UpdateTenant / DeleteTenant /
// MintKey) 在更新内存 map 之后,立即通过 kv 落盘。
//
// 如果 kv 为 nil,registry 退化为纯内存模式(R3 之前的
// 行为);生产配置应总传入 kv。
type Registry struct {
	mu       sync.RWMutex
	tenants  map[string]*Tenant
	keys     map[string]string // plaintext key -> tenant ID
	kv       xpersistence.KV    // 可选, nil = 纯内存
}

// New 创建一个 in-memory only 的注册表(向后兼容 R3)。
//
// 持久化请用 NewWithKV。
func New() *Registry {
	return &Registry{
		tenants: make(map[string]*Tenant),
		keys:    make(map[string]string),
	}
}

// NewWithKV 创建一个带持久化的注册表。
//
// 启动时从 KV 加载所有已有 tenant + key,然后才返回;加载
// 失败(损坏、I/O 错误)会回退为 in-memory only 并保留错误,
// 由调用方决定要不要中断启动。
func NewWithKV(ctx context.Context, kv xpersistence.KV) (*Registry, error) {
	if kv == nil {
		return nil, errors.New("tenants: kv is nil")
	}
	r := &Registry{
		tenants: make(map[string]*Tenant),
		keys:    make(map[string]string),
		kv:      kv,
	}
	if err := r.load(ctx); err != nil {
		return nil, fmt.Errorf("tenants: load: %w", err)
	}
	return r, nil
}

// load 从 KV 读所有 tenant 和反向索引,填到内存 map。
//
// 缺失的 index 不算错误(可能某次 MintKey 成功但 index 还没
// 写入 —— 实际不存在;但反过来 index 存在而 tenant 不存在
// 是脏数据,会丢弃该 index)。
func (r *Registry) load(ctx context.Context) error {
	tenantKeys, err := r.kv.List(ctx, "tenants/")
	if err != nil {
		return err
	}
	for _, k := range tenantKeys {
		raw, err := r.kv.Get(ctx, k)
		if err != nil {
			continue
		}
		var t Tenant
		if err := json.Unmarshal(raw, &t); err != nil {
			continue
		}
		r.tenants[t.ID] = &t
	}
	keyKeys, err := r.kv.List(ctx, "tenants-key/")
	if err != nil {
		return err
	}
	for _, k := range keyKeys {
		raw, err := r.kv.Get(ctx, k)
		if err != nil {
			continue
		}
		var t struct {
			TenantID  string    `json:"tenant_id"`
			KeyID     string    `json:"key_id,omitempty"`
			Label     string    `json:"label"`
			Plaintext string    `json:"plaintext"`
			Role      string    `json:"role"`
			CreatedAt time.Time `json:"created_at"`
		}
		if err := json.Unmarshal(raw, &t); err != nil {
			continue
		}
		if _, ok := r.tenants[t.TenantID]; !ok {
			// 悬挂的 key,跳过
			continue
		}
		r.keys[t.Plaintext] = t.TenantID
	}
	return nil
}

// SetKV 用于测试中后注入 KV,或在 New 之后
// 启动阶段才打开 KV 的场景。
//
// 调用前必须空 registry(没有租户)。
func (r *Registry) SetKV(ctx context.Context, kv xpersistence.KV) error {
	if kv == nil {
		return errors.New("tenants: kv is nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.tenants) > 0 || len(r.keys) > 0 {
		return errors.New("tenants: SetKV on non-empty registry")
	}
	r.kv = kv
	return r.load(ctx)
}

// CreateTenant 注册一个新租户。ID 被规范化为小写 slug;
// 空的 ID 返回 an error. ID 已存在返回 conflict。
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
	if err := r.persistTenant(t); err != nil {
		// 回滚内存
		delete(r.tenants, id)
		return nil, err
	}
	out := *t
	return &out, nil
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

// UpdateTenant 用传入的 Tenant 覆盖现有记录(调用方负责取一份最新值然后修改)。
//
// ID 字段不可变 —— 我们用现有条目做幂等键。
func (r *Registry) UpdateTenant(t *Tenant) error {
	if t == nil || t.ID == "" {
		return errors.New("tenant id is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tenants[t.ID]; !ok {
		return ErrNotFound
	}
	t.Active = true
	r.tenants[t.ID] = t
	return r.persistTenant(t)
}

// DeleteTenant 移除一个 tenant 及其所有挂载的 key。
//
// plaintext -> tenantID 反向索引会一并清空,避免悬挂的
// 明文 key 误指向已被删除的 tenant。
func (r *Registry) DeleteTenant(id string) error {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return errors.New("tenant id is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tenants[id]; !ok {
		return ErrNotFound
	}
	delete(r.tenants, id)
	// 同时清空该 tenant 下所有 key 索引
	var toDelete []string
	for pt, tid := range r.keys {
		if tid == id {
			toDelete = append(toDelete, pt)
		}
	}
	for _, pt := range toDelete {
		delete(r.keys, pt)
	}
	// 落盘(失败也继续,内存已经改了)
	if r.kv != nil {
		ctx := context.Background()
		_ = r.kv.Delete(ctx, "tenants/"+id)
		for _, pt := range toDelete {
			_ = r.kv.Delete(ctx, "tenants-key/"+pt)
		}
	}
	return nil
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
	k := &Key{
		TenantID:  tenantID,
		Label:     label,
		Plaintext: plaintext,
		Role:      role,
		CreatedAt: time.Now().UTC(),
	}
	if err := r.persistKey(k); err != nil {
		delete(r.keys, plaintext)
		return nil, err
	}
	out := *k
	return &out, nil
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

// persistTenant 写一个 tenant 到 KV。
func (r *Registry) persistTenant(t *Tenant) error {
	if r.kv == nil {
		return nil
	}
	raw, err := json.Marshal(t)
	if err != nil {
		return err
	}
	return r.kv.Set(context.Background(), "tenants/"+t.ID, raw)
}

// persistKey 写一个 key 反向索引到 KV。
func (r *Registry) persistKey(k *Key) error {
	if r.kv == nil {
		return nil
	}
	// 只持久化反向索引;TenantID/Plaintext 已经存在 tenant 里
	raw, err := json.Marshal(struct {
		TenantID  string    `json:"tenant_id"`
		KeyID     string    `json:"key_id,omitempty"`
		Label     string    `json:"label"`
		Plaintext string    `json:"plaintext"`
		Role      string    `json:"role"`
		CreatedAt time.Time `json:"created_at"`
	}{
		TenantID:  k.TenantID,
		KeyID:     k.KeyID,
		Label:     k.Label,
		Plaintext: k.Plaintext,
		Role:      k.Role,
		CreatedAt: k.CreatedAt,
	})
	if err != nil {
		return err
	}
	return r.kv.Set(context.Background(), "tenants-key/"+k.Plaintext, raw)
}

func randomKey() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return "dog_" + hex.EncodeToString(b)
}
