package auth

// admin_store.go：AdminStore 主体。
//
// AdminStore 持有 API key 表，支持 CRUD、轮换、按 token/ID 查找、过期清理。
// 使用 sync.RWMutex 保护 hash 表；使用 atomic.Int64 生成自增计数器。

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
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
}

// NewAdminStore 创建一个空的 AdminStore。
func NewAdminStore() *AdminStore {
	return &AdminStore{
		keys:     make(map[string]*KeyEntry),
		hashToID: make(map[string]string),
	}
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
	return nil
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
	for id, e := range s.keys {
		if !e.ExpiresAt.IsZero() && now.After(e.ExpiresAt) {
			delete(s.hashToID, e.Hash)
			delete(s.keys, id)
			n++
		}
	}
	return n
}
