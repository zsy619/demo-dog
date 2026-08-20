// Package quota 配额管理器：按 key 维护容量配额，支持租户隔离。
package quota

import (
	"errors"
	"sync"
	"time"
)

// Bucket 表示某个租户的令牌桶。
type Bucket struct {
	Tenant    string
	Capacity  float64
	Refill    float64 // tokens per second
	Tokens    float64
	LastRefil time.Time
}

// Allow 在可消费一个令牌时返回 true；
// 它会根据经过的时间补充令牌。
func (b *Bucket) Allow(now time.Time) bool {
	elapsed := now.Sub(b.LastRefil).Seconds()
	if elapsed > 0 {
		b.Tokens += elapsed * b.Refill
		if b.Tokens > b.Capacity {
			b.Tokens = b.Capacity
		}
		b.LastRefil = now
	}
	if b.Tokens >= 1 {
		b.Tokens--
		return true
	}
	return false
}

// ErrTenantNotConfigured 在对一个未配置桶的租户调用 Allow 时返回。
var ErrTenantNotConfigured = errors.New("tenant quota not configured")

// Manager 管理各个租户的桶。
type Manager struct {
	mu       sync.RWMutex
	buckets  map[string]*Bucket
	defaults Bucket
}

// NewManager 返回一个 Manager，并为没有
// 单独覆盖的租户提供默认桶。
func NewManager(defaultCap, defaultRefill float64) *Manager {
	return &Manager{
		buckets: make(map[string]*Bucket),
		defaults: Bucket{
			Capacity: defaultCap, Refill: defaultRefill,
			Tokens: defaultCap, LastRefil: time.Now(),
		},
	}
}

// Set 配置或替换一个租户桶。
func (m *Manager) Set(tenant string, cap, refill float64) {
	m.mu.Lock()
	m.buckets[tenant] = &Bucket{
		Tenant: tenant, Capacity: cap, Refill: refill,
		Tokens: cap, LastRefil: time.Now(),
	}
	m.mu.Unlock()
}

// Remove 删除某个租户的自定义配置（回退到默认值）。
func (m *Manager) Remove(tenant string) {
	m.mu.Lock()
	delete(m.buckets, tenant)
	m.mu.Unlock()
}

// Allow 从该租户的桶中消费一个令牌。
func (m *Manager) Allow(tenant string) (bool, error) {
	m.mu.RLock()
	b, ok := m.buckets[tenant]
	if !ok {
		b = &m.defaults
	}
	m.mu.RUnlock()
	if b.Capacity == 0 && b.Refill == 0 {
		return false, ErrTenantNotConfigured
	}
	return b.Allow(time.Now()), nil
}

// Tokens 返回当前令牌数量（经过虚拟补充后）。
// 用于可观测性。
func (m *Manager) Tokens(tenant string) float64 {
	m.mu.RLock()
	b, ok := m.buckets[tenant]
	if !ok {
		b = &m.defaults
	}
	m.mu.RUnlock()
	now := time.Now()
	elapsed := now.Sub(b.LastRefil).Seconds()
	t := b.Tokens + elapsed*b.Refill
	if t > b.Capacity {
		t = b.Capacity
	}
	return t
}

// Stats 表示单个租户的当前状态。
type Stats struct {
	Tenant   string  `json:"tenant"`
	Capacity float64 `json:"capacity"`
	Refill   float64 `json:"refill"`
	Tokens   float64 `json:"tokens"`
}

// Snapshot 为每个已配置的租户返回一条 Stats，
// 同时包含默认桶的条目。
func (m *Manager) Snapshot() []Stats {
	m.mu.RLock()
	tenants := make([]string, 0, len(m.buckets))
	for k := range m.buckets {
		tenants = append(tenants, k)
	}
	m.mu.RUnlock()
	out := make([]Stats, 0, len(tenants)+1)
	for _, t := range tenants {
		m.mu.RLock()
		b := m.buckets[t]
		m.mu.RUnlock()
		out = append(out, Stats{
			Tenant: t, Capacity: b.Capacity, Refill: b.Refill,
			Tokens: m.Tokens(t),
		})
	}
	out = append(out, Stats{
		Tenant: "_default", Capacity: m.defaults.Capacity,
		Refill: m.defaults.Refill, Tokens: m.Tokens("_default"),
	})
	return out
}
