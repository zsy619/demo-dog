// Package lease 租约管理：自动续约与过期检测。
package lease

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// Lease 授予一个持有者对命名资源的独占访问权
// for a bounded duration. The manager tracks active leases and
// lets callers Renew / Release them.
type Lease struct {
	Name      string
	Holder    string
	ID        string
	ExpiresAt time.Time
	acquired  time.Time
}

// Active 报告租约是否仍在有效期内。
func (l *Lease) Active(now time.Time) bool {
	return !l.ExpiresAt.IsZero() && now.Before(l.ExpiresAt)
}

// Manager 拥有租约表。
type Manager struct {
	mu       sync.Mutex
	leases   map[string]*Lease
	now      func() time.Time
	duration time.Duration
}

// New 以默认租约时长创建一个 Manager。
func New(d time.Duration) *Manager {
	if d <= 0 {
		d = 30 * time.Second
	}
	return &Manager{
		leases:   make(map[string]*Lease),
		duration: d,
		now:      time.Now,
	}
}

// WithTime overrides the time source for tests.
func (m *Manager) WithTime(now func() time.Time) *Manager {
	m.now = now
	return m
}

// ErrHeld 在另一个持有者持有租约时返回。
var ErrHeld = errors.New("lease held by another holder")

// Acquire takes the lease for the named holder. If the lease
// is free or expired, returns a new lease; otherwise returns
// ErrHeld.
func (m *Manager) Acquire(name, holder string) (*Lease, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	if existing, ok := m.leases[name]; ok && existing.Active(now) {
		return nil, ErrHeld
	}
	id := newID()
	l := &Lease{
		Name: name, Holder: holder, ID: id,
		ExpiresAt: now.Add(m.duration),
		acquired:  now,
	}
	m.leases[name] = l
	return l, nil
}

// Renew extends the lease for an additional duration.
// Returns ErrHeld if the lease is not owned by holder.
func (m *Manager) Renew(name, holder string) (*Lease, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.leases[name]
	if !ok || l.Holder != holder {
		return nil, ErrHeld
	}
	l.ExpiresAt = m.now().Add(m.duration)
	return l, nil
}

// Release frees the lease if the holder matches.
func (m *Manager) Release(name, holder string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.leases[name]
	if !ok {
		return nil
	}
	if l.Holder != holder {
		return ErrHeld
	}
	delete(m.leases, name)
	return nil
}

// Get returns the active lease for name, or nil.
func (m *Manager) Get(name string) *Lease {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.leases[name]
	if !ok || !l.Active(m.now()) {
		return nil
	}
	return l
}

// Sweep 淘汰过期租约。返回租约数量
// removed. Callers can run this on a timer.
func (m *Manager) Sweep() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	n := 0
	for k, l := range m.leases {
		if !l.Active(now) {
			delete(m.leases, k)
			n++
		}
	}
	return n
}

// Active 返回未过期租约的数量。
func (m *Manager) Active() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	n := 0
	for _, l := range m.leases {
		if l.Active(now) {
			n++
		}
	}
	return n
}

// Names 返回所有当前持有的租约名称。
func (m *Manager) Names() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	out := make([]string, 0, len(m.leases))
	for k, l := range m.leases {
		if l.Active(now) {
			out = append(out, k)
		}
	}
	return out
}

func newID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
