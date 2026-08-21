package auth

// admin_store.go：AdminStore 主体。
//
// AdminStore 持有 API key 表，支持 CRUD、轮换、按 token/ID 查找、过期清理。
// 使用 sync.RWMutex 保护 hash 表；使用 atomic.Int64 生成自增计数器。
//
// W1 起,AdminStore 可选挂一个 xpersistence.KV,所有 CRUD
// 都会通过写穿(write-through)方式落盘,进程重启后保留所有
// API key。不传 KV 则退化为纯内存(R3 之前的行为)。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xpersistence"
)

// AdminStore 持有 API key 表（线程安全）。
//
// 内部维护两个索引：
//   - keys：按 KeyID 索引；
//   - hashToID：按 raw token 的 sha256 索引（便于 LookupByToken）。
type AdminStore struct {
	mu       sync.RWMutex    // 保护 keys / hashToID
	keys     map[string]*KeyEntry // KeyID → 条目
	hashToID map[string]string   // token hash → KeyID
	counter  atomic.Int64    // 用于生成唯一 KeyID 的自增计数器
	kv       xpersistence.KV  // 可选,nil = 纯内存
}

// NewAdminStore 创建一个空的 AdminStore。
//
// 持久化请用 NewAdminStoreWithKV。
func NewAdminStore() *AdminStore {
	return &AdminStore{
		keys:     make(map[string]*KeyEntry),
		hashToID: make(map[string]string),
	}
}

// NewAdminStoreWithKV 创建一个带持久化的 AdminStore。
//
// 启动时从 KV 加载所有 KeyEntry;加载失败(损坏、I/O 错误)
// 返回错误,由调用方决定是否中断启动。
func NewAdminStoreWithKV(ctx context.Context, kv xpersistence.KV) (*AdminStore, error) {
	if kv == nil {
		return nil, errors.New("auth: kv is nil")
	}
	s := &AdminStore{
		keys:     make(map[string]*KeyEntry),
		hashToID: make(map[string]string),
		kv:       kv,
	}
	if err := s.load(ctx); err != nil {
		return nil, fmt.Errorf("auth: load: %w", err)
	}
	return s, nil
}

// SetKV 用于测试或后注入 KV。调用前必须空 store。
func (s *AdminStore) SetKV(ctx context.Context, kv xpersistence.KV) error {
	if kv == nil {
		return errors.New("auth: kv is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.keys) > 0 {
		return errors.New("auth: SetKV on non-empty store")
	}
	s.kv = kv
	return s.load(ctx)
}

// load 从 KV 读所有 key entries,填到内存索引。
func (s *AdminStore) load(ctx context.Context) error {
	keyNames, err := s.kv.List(ctx, "admin-keys/")
	if err != nil {
		return err
	}
	for _, k := range keyNames {
		raw, err := s.kv.Get(ctx, k)
		if err != nil {
			continue
		}
		var e KeyEntry
		if err := json.Unmarshal(raw, &e); err != nil {
			continue
		}
		if e.KeyID == "" || e.Hash == "" {
			continue
		}
		// 跳过已过期的 key(启动时不主动清理,等 PurgeExpired)
		if !e.ExpiresAt.IsZero() && time.Now().After(e.ExpiresAt) {
			continue
		}
		s.keys[e.KeyID] = &e
		s.hashToID[e.Hash] = e.KeyID
		// 维护 counter 不退化(用 KeyID 后缀时间戳取最大值)
		// 实际中 nextID 用 nanos 区分,counter 主要防止 nanos 撞车,
		// 不强制同步到 max,这里跳过。
	}
	return nil
}

// persist 写一个 entry 到 KV。
func (s *AdminStore) persist(e *KeyEntry) error {
	if s.kv == nil {
		return nil
	}
	raw, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return s.kv.Set(context.Background(), "admin-keys/"+e.KeyID, raw)
}

// remove 删除一个 entry 的 KV 记录。
func (s *AdminStore) remove(id string) {
	if s.kv == nil {
		return
	}
	_ = s.kv.Delete(context.Background(), "admin-keys/"+id)
}

// nextID 生成一个新的 KeyID（包含时间戳 + 自增计数器，保证唯一）。
//
// 必须在持锁状态下调用。
func (s *AdminStore) nextID() string {
	n := s.counter.Add(1)
	return fmt.Sprintf("key-%d-%d", time.Now().UnixNano(), n)
}

// CreateKey 生成新的 API key。
//
// 返回原始 token（仅在创建时可见）与持久化的 KeyEntry。
// ttl > 0 时设置过期时间；ttl == 0 表示永不过期。
//
// R3: 新增 label 参数——之前 label 总是被填成 role,导致
// 前端 UI 上两列永远相等。label 为空时回退到 identity 兼容旧调用方。
func (s *AdminStore) CreateKey(identity, label, tenant string, scopes []string, ttl time.Duration) (raw string, entry *KeyEntry, err error) {
	raw, err = GenerateToken()
	if err != nil {
		return "", nil, err
	}
	if label == "" {
		label = identity
	}
	entry = &KeyEntry{
		KeyID:     s.nextID(),
		Hash:      hashToken(raw),
		Identity:  identity,
		Label:     label,
		Tenant:    tenant,
		Scopes:    scopes,
		CreatedAt: time.Now(),
	}
	if ttl > 0 {
		entry.ExpiresAt = time.Now().Add(ttl)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys[entry.KeyID] = entry
	s.hashToID[entry.Hash] = entry.KeyID
	if err := s.persist(entry); err != nil {
		// 回滚内存
		delete(s.hashToID, entry.Hash)
		delete(s.keys, entry.KeyID)
		return "", nil, err
	}
	return raw, entry, nil
}

// LookupByToken 根据原始 token 返回对应的 KeyEntry。
//
// token 不匹配任何条目时返回 (nil, false)。
func (s *AdminStore) LookupByToken(raw string) (*KeyEntry, bool) {
	h := hashToken(raw)
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.hashToID[h]
	if !ok {
		return nil, false
	}
	return s.keys[id], true
}

// LookupByID 按 ID 查找 KeyEntry。
func (s *AdminStore) LookupByID(id string) (*KeyEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.keys[id]
	return e, ok
}

// RotateKey 颁发一个新 key（保持原 identity/tenant/scopes），
// 并把旧 key 标记为 grace 后过期。
//
// 返回新 raw token、旧条目、新条目。
// oldID 不存在时返回错误。
func (s *AdminStore) RotateKey(oldID string, grace time.Duration) (raw string, oldEntry, newEntry *KeyEntry, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.keys[oldID]
	if !ok {
		return "", nil, nil, errors.New("key not found")
	}
	raw, err = GenerateToken()
	if err != nil {
		return "", nil, nil, err
	}
	newEntry = &KeyEntry{
		KeyID:       s.nextID(),
		Hash:        hashToken(raw),
		Identity:    old.Identity,
		Label:       old.Label,
		Tenant:      old.Tenant,
		Scopes:      old.Scopes,
		CreatedAt:   time.Now(),
		RotatedFrom: old.KeyID,
	}
	if grace > 0 {
		old.ExpiresAt = time.Now().Add(grace)
	}
	s.keys[newEntry.KeyID] = newEntry
	s.hashToID[newEntry.Hash] = newEntry.KeyID
	if err := s.persist(newEntry); err != nil {
		// 回滚内存
		delete(s.hashToID, newEntry.Hash)
		delete(s.keys, newEntry.KeyID)
		return "", nil, nil, err
	}
	// 旧 key 的 ExpiresAt 已被设置,落盘
	if grace > 0 {
		if perr := s.persist(old); perr != nil {
			// 即使旧 key 落盘失败也不影响新 key;调用方后续可重试
			_ = perr
		}
	}
	return raw, old, newEntry, nil
}

// DisableKey 标记 key 为禁用；后续查找返回 false。
func (s *AdminStore) DisableKey(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.keys[id]
	if !ok {
		return errors.New("key not found")
	}
	e.Disabled = true
	return s.persist(e)
}

// DeleteKey 永久删除一个 key。
func (s *AdminStore) DeleteKey(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.keys[id]
	if !ok {
		return errors.New("key not found")
	}
	delete(s.hashToID, e.Hash)
	delete(s.keys, id)
	s.remove(id)
	return nil
}

// ListKeys 返回所有条目（顺序未指定）。
func (s *AdminStore) ListKeys() []*KeyEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*KeyEntry, 0, len(s.keys))
	for _, e := range s.keys {
		out = append(out, e)
	}
	return out
}

// PurgeExpired 删除 ExpiresAt 已过的 key。
//
// 返回删除的条目数。
func (s *AdminStore) PurgeExpired(now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	var toRemove []string
	for id, e := range s.keys {
		if !e.ExpiresAt.IsZero() && now.After(e.ExpiresAt) {
			delete(s.hashToID, e.Hash)
			delete(s.keys, id)
			toRemove = append(toRemove, id)
			n++
		}
	}
	for _, id := range toRemove {
		s.remove(id)
	}
	return n
}
