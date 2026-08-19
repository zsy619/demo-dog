package quota

import (
	"errors"
	"sync"
	"time"
)

// Bucket is one tenant's token bucket.
type Bucket struct {
	Tenant    string
	Capacity  float64
	Refill    float64 // tokens per second
	Tokens    float64
	LastRefil time.Time
}

// Allow returns true when one token can be consumed; it
// refills the bucket based on elapsed time.
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

// ErrTenantNotConfigured is returned when Allow is called for
// a tenant without a configured bucket.
var ErrTenantNotConfigured = errors.New("tenant quota not configured")

// Manager owns per-tenant buckets.
type Manager struct {
	mu       sync.RWMutex
	buckets  map[string]*Bucket
	defaults Bucket
}

// NewManager returns a Manager with a default bucket for
// tenants that have no per-tenant override.
func NewManager(defaultCap, defaultRefill float64) *Manager {
	return &Manager{
		buckets: make(map[string]*Bucket),
		defaults: Bucket{
			Capacity: defaultCap, Refill: defaultRefill,
			Tokens: defaultCap, LastRefil: time.Now(),
		},
	}
}

// Set configures or replaces a tenant bucket.
func (m *Manager) Set(tenant string, cap, refill float64) {
	m.mu.Lock()
	m.buckets[tenant] = &Bucket{
		Tenant: tenant, Capacity: cap, Refill: refill,
		Tokens: cap, LastRefil: time.Now(),
	}
	m.mu.Unlock()
}

// Remove drops a tenant override (falls back to default).
func (m *Manager) Remove(tenant string) {
	m.mu.Lock()
	delete(m.buckets, tenant)
	m.mu.Unlock()
}

// Allow consumes one token from the tenant's bucket.
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

// Tokens returns the current token count (after a virtual
// refill). For observability.
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

// Stats returns per-tenant current state.
type Stats struct {
	Tenant   string  `json:"tenant"`
	Capacity float64 `json:"capacity"`
	Refill   float64 `json:"refill"`
	Tokens   float64 `json:"tokens"`
}

// Snapshot returns one Stats entry per configured tenant +
// the default.
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
