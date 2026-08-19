package lease

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// Lease grants one holder exclusive access to a named resource
// for a bounded duration. The manager tracks active leases and
// lets callers Renew / Release them.
type Lease struct {
	Name      string
	Holder    string
	ID        string
	ExpiresAt time.Time
	acquired  time.Time
}

// Active reports whether the lease is still in its window.
func (l *Lease) Active(now time.Time) bool {
	return !l.ExpiresAt.IsZero() && now.Before(l.ExpiresAt)
}

// Manager owns the lease table.
type Manager struct {
	mu       sync.Mutex
	leases   map[string]*Lease
	now      func() time.Time
	duration time.Duration
}

// New creates a Manager with the default lease duration.
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

// ErrHeld is returned when another holder owns the lease.
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

// Sweep evicts expired leases. Returns the number of leases
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

// Active returns the number of non-expired leases.
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

// Names returns all currently-held lease names.
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
