package api

// api_key.go：APIKeyAuth 主体实现。
//
// 包含注册、查询、删除、范围校验等所有 KeyEntry 的 CRUD 与能力判定逻辑。
// 使用 sync.RWMutex 保护内部 map；常量时间比对避免时序侧信道。

import (
	"crypto/subtle"
	"errors"
	"strings"
	"sync"
)

// APIKeyAuth 是一个轻量的 API Key 注册表。
//
// 查找使用 crypto/subtle.ConstantTimeCompare（对等长度的候选对），
// 防止通过时序差异泄漏信息。
type APIKeyAuth struct {
	mu      sync.RWMutex             // 保护所有 map 的读写锁
	keys    map[string]struct{}      // 已注册 key 集合
	labels  map[string]string        // key → 标签
	roles   map[string]Role          // key → 角色
	tenants map[string]string        // key → 租户
	scopes  map[string][]string      // key → 资源范围
}

// NewAPIKeyAuth 返回一个空的鉴权注册表。
func NewAPIKeyAuth() *APIKeyAuth {
	return &APIKeyAuth{
		keys:    make(map[string]struct{}),
		labels:  make(map[string]string),
		roles:   make(map[string]Role),
		tenants: make(map[string]string),
		scopes:  make(map[string][]string),
	}
}

// Add 注册一个新的 key，包含可选的标签、角色与租户绑定。
//
// 空 key 会被忽略，避免调用方误创建通配符。
// 当 role 未提供时默认为 RoleWriter，这样最常见场景（SDK 上报遥测）开箱即用。
// 当 tenant 为空时，该 key 可作用于任意租户（平台管理员风格）。
func (a *APIKeyAuth) Add(key, label string, role ...Role) {
	if key == "" {
		return
	}
	r := RoleWriter
	if len(role) > 0 {
		r = role[0]
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.keys[key] = struct{}{}
	a.labels[key] = label
	a.roles[key] = r
}

// AddForTenant 注册一个绑定到指定租户的 key。
func (a *APIKeyAuth) AddForTenant(key, label, tenant string, role ...Role) {
	a.Add(key, label, role...)
	a.mu.Lock()
	defer a.mu.Unlock()
	a.tenants[key] = tenant
}

// AddWithScopes 与 Add 类似，额外附加资源范围列表。
//
// 该 key 只允许访问名字出现在 scopes 列表中的资源。
func (a *APIKeyAuth) AddWithScopes(key, label, tenant string, role Role, scopes []string) {
	if key == "" {
		return
	}
	r := role
	if r == 0 {
		r = RoleWriter
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.keys[key] = struct{}{}
	a.labels[key] = label
	a.roles[key] = r
	a.tenants[key] = tenant
	a.scopes[key] = scopes
}

// AddFromSpec 接受形如 "<key>:<role>:<label>:<tenant>"、"<key>:<role>:<label>"、
// "<key>:<role>" 或 "<key>" 的字符串。
//
// 这是 CLI flag parser 从 `-api-keys k1:admin:alice:acme,k2:writer:checkout:acme,k3`
// 解析出来的格式。未知角色 fallback 到 RoleWriter。
func (a *APIKeyAuth) AddFromSpec(spec string) {
	parts := strings.SplitN(spec, ":", 4)
	switch len(parts) {
	case 1:
		a.Add(parts[0], "", RoleWriter)
	case 2:
		a.Add(parts[0], "", ParseRole(parts[1]))
	case 3:
		a.Add(parts[0], parts[2], ParseRole(parts[1]))
	default:
		a.Add(parts[0], parts[2], ParseRole(parts[1]))
		a.mu.Lock()
		defer a.mu.Unlock()
		a.tenants[parts[0]] = parts[3]
	}
}

// Remove 从注册表中删除一个 key。
func (a *APIKeyAuth) Remove(key string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.keys, key)
	delete(a.labels, key)
	delete(a.roles, key)
	delete(a.tenants, key)
	delete(a.scopes, key)
}

// Count 返回已注册 key 的数量。
func (a *APIKeyAuth) Count() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.keys)
}

// Verify 返回 key 是否匹配某个已注册的 key。
//
// 对相同长度的候选 key 使用 crypto/subtle.ConstantTimeCompare，
// 以抵抗时序侧信道攻击；空集合始终返回 false。
func (a *APIKeyAuth) Verify(key string) bool {
	if key == "" {
		return false
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	for candidate := range a.keys {
		if len(candidate) != len(key) {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(key)) == 1 {
			return true
		}
	}
	return false
}

// RoleOf 返回 key 关联的角色。
//
// key 未注册时返回 false；调用方应先调 Verify，
// 仅在已知 key 时再分支处理角色。
func (a *APIKeyAuth) RoleOf(key string) (Role, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	r, ok := a.roles[key]
	return r, ok
}

// LabelOf 返回 key 对应的人类可读标签。
func (a *APIKeyAuth) LabelOf(key string) string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.labels[key]
}

// List 返回所有已注册 key 的快照。返回切片对调用方而言可安全修改。
func (a *APIKeyAuth) List() []KeyEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]KeyEntry, 0, len(a.keys))
	for k := range a.keys {
		out = append(out, KeyEntry{
			Key:      k,
			Label:    a.labels[k],
			Role:     a.roles[k],
			TenantID: a.tenants[k],
		})
	}
	return out
}

// TenantOf 返回 key 的租户绑定，未绑定时返回空（即可作用于任意租户）。
func (a *APIKeyAuth) TenantOf(key string) string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.tenants[key]
}

// lookupLocked 返回 key 的完整条目。
//
// 必须在持有锁的状态下调用。
func (a *APIKeyAuth) lookupLocked(key string) (*KeyEntry, bool) {
	if _, ok := a.keys[key]; !ok {
		return nil, false
	}
	return &KeyEntry{
		Key:      key,
		Label:    a.labels[key],
		Role:     a.roles[key],
		TenantID: a.tenants[key],
		Scopes:   a.scopes[key],
	}, true
}

// Lookup 返回 key 的完整条目（含 scopes）。
func (a *APIKeyAuth) Lookup(key string) (KeyEntry, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	e, ok := a.lookupLocked(key)
	if !ok {
		return KeyEntry{}, false
	}
	return *e, true
}

// AllowsResource 报告 key 是否被允许访问指定名字的资源。
//
// 空的 Scopes 列表允许租户内全部资源；非空列表仅允许列表内资源。
func (a *APIKeyAuth) AllowsResource(key, resource string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	e, ok := a.lookupLocked(key)
	if !ok {
		return false
	}
	if len(e.Scopes) == 0 {
		return true
	}
	for _, s := range e.Scopes {
		if s == resource {
			return true
		}
	}
	return false
}

// ScopesFor 返回 key 关联的范围列表。
//
// key 未注册时返回 nil。
func (a *APIKeyAuth) ScopesFor(key string) []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	e, ok := a.lookupLocked(key)
	if !ok {
		return nil
	}
	if len(e.Scopes) == 0 {
		return nil
	}
	out := make([]string, len(e.Scopes))
	copy(out, e.Scopes)
	return out
}

// ErrTenantMismatch 表示某个 API key 试图访问其未被授权的租户。
//
// W2.1: 跨租户调用必须显式拒绝,以防某个租户的 key 通过 ?tenant=xxx
// 或 X-Tenant-Id 头部越权读取其他租户的数据。
var ErrTenantMismatch = errors.New("api key not authorized for tenant")

// IsTenantMismatch 报告 err 是否来自 EnsureTenantAccess。
func IsTenantMismatch(err error) bool { return errors.Is(err, ErrTenantMismatch) }

// EnsureTenantAccess 校验 key 在请求 claimedTenant 上的访问权。
//
// 规则:
//   - key 未注册 → ErrUnauthorized 语义(交给调用方包装,这里返回通用错误)。
//   - key.TenantID == "" (平台管理员 key) → 任意 claimedTenant 都允许。
//   - key.Role == RoleAdmin → 任意 claimedTenant 都允许(管理员运维场景)。
//   - key.TenantID == claimedTenant → 允许。
//   - 其他 → ErrTenantMismatch,调用方应返回 403。
//
// claimedTenant 为空时表示调用方未声明租户(典型场景:列出全部租户的
// 管理端点),只有平台 / 管理员 key 可放行。
func (a *APIKeyAuth) EnsureTenantAccess(key, claimedTenant string) error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	e, ok := a.lookupLocked(key)
	if !ok {
		return errors.New("api key not found")
	}
	if e.TenantID == "" || e.Role == RoleAdmin {
		return nil
	}
	if claimedTenant == "" {
		return ErrTenantMismatch
	}
	if e.TenantID != claimedTenant {
		return ErrTenantMismatch
	}
	return nil
}

// TenantOfLocked 假设 key 已 Verify/lookup 过,无锁地返回 TenantID。
//
// 调用方负责保证:已持锁 或 已通过 Verify 确认 key 存在。
// 主要给中间件用——Verify 已锁过一次,直接走 map 读取更高效。
func (a *APIKeyAuth) TenantOfLocked(key string) string {
	return a.tenants[key]
}
