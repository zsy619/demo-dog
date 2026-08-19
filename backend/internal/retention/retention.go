package retention

// Per-tenant retention policies + cold-storage eviction.
//
// Different tenants pay for different retention tiers. A
// free tier keeps 1 day of hot logs and 7 days of cold.
// A pro tier keeps 7 days hot + 30 days cold. An enterprise
// tier is unlimited. The Policy table is read by the Doris
// eviction sweep on every tick.
//
// Cold storage is modelled as a separate directory; the
// sweeper moves bytes out of hot into cold once they age
// past the hot TTL, then drops them once they age past the
// cold TTL.
//
// The interface for the sweeper is small: it asks the store
// "give me one tenant row at a time" and "drop these log
// rows" or "move them here".

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Tier names a retention profile.
type Tier string

const (
	TierFree       Tier = "free"
	TierPro        Tier = "pro"
	TierEnterprise Tier = "enterprise"
)

// Policy is the per-tenant retention setting.
type Policy struct {
	Tenant    string
	Tier      Tier
	HotTTL    time.Duration
	ColdTTL   time.Duration
	UpdatedAt time.Time
}

// DefaultPolicies returns the canonical tier settings.
func DefaultPolicies() map[Tier]Policy {
	return map[Tier]Policy{
		TierFree:       {Tier: TierFree, HotTTL: 24 * time.Hour, ColdTTL: 7 * 24 * time.Hour},
		TierPro:        {Tier: TierPro, HotTTL: 7 * 24 * time.Hour, ColdTTL: 30 * 24 * time.Hour},
		TierEnterprise: {Tier: TierEnterprise, HotTTL: 90 * 24 * time.Hour, ColdTTL: 365 * 24 * time.Hour},
	}
}

// Manager owns the per-tenant policies + runs the sweeper.
type Manager struct {
	mu       sync.RWMutex
	policies map[string]Policy
	coldDir  string
	sweeped  int64
	moved    int64
	bytes    int64
	now      func() time.Time
}

// NewManager returns an empty manager.
func NewManager(coldDir string, now func() time.Time) *Manager {
	if now == nil {
		now = time.Now
	}
	return &Manager{
		policies: make(map[string]Policy),
		coldDir:  coldDir,
		now:      now,
	}
}

// Set assigns a tenant to a tier (overwriting any prior
// setting).
func (m *Manager) Set(tenant string, tier Tier) {
	defs := DefaultPolicies()
	tmpl, ok := defs[tier]
	if !ok {
		tmpl = defs[TierFree]
	}
	p := tmpl
	p.Tenant = tenant
	p.UpdatedAt = m.now()
	m.mu.Lock()
	m.policies[tenant] = p
	m.mu.Unlock()
}

// SetPolicy lets the caller specify a custom policy (e.g.
// regulatory hold). HotTTL/ColdTTL must be positive for the
// policy to evict anything.
func (m *Manager) SetPolicy(p Policy) error {
	if p.Tenant == "" {
		return errors.New("tenant required")
	}
	if p.HotTTL < 0 || p.ColdTTL < 0 {
		return errors.New("ttl must be non-negative")
	}
	if p.HotTTL > p.ColdTTL {
		return errors.New("hot TTL cannot exceed cold TTL")
	}
	p.UpdatedAt = m.now()
	m.mu.Lock()
	m.policies[p.Tenant] = p
	m.mu.Unlock()
	return nil
}

// Get returns the policy for a tenant.
func (m *Manager) Get(tenant string) (Policy, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.policies[tenant]
	return p, ok
}

// Remove deletes the policy.
func (m *Manager) Remove(tenant string) {
	m.mu.Lock()
	delete(m.policies, tenant)
	m.mu.Unlock()
}

// List returns the current policies sorted by tenant name.
func (m *Manager) List() []Policy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Policy, 0, len(m.policies))
	for _, p := range m.policies {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Tenant < out[j].Tenant })
	return out
}

// Decision tells the caller what to do with a single log row.
type Decision struct {
	Tenant string
	Age    time.Duration
	Action string // keep, move_to_cold, drop
}

// Decide inspects one log row and tells the caller the action
// to take. age is (now - row.Timestamp).
func (m *Manager) Decide(tenant string, age time.Duration) Decision {
	m.mu.RLock()
	p, ok := m.policies[tenant]
	m.mu.RUnlock()
	d := Decision{Tenant: tenant, Age: age}
	if !ok {
		d.Action = "keep"
		return d
	}
	switch {
	case age > p.ColdTTL && p.ColdTTL > 0:
		d.Action = "drop"
	case age > p.HotTTL && p.HotTTL > 0:
		d.Action = "move_to_cold"
	default:
		d.Action = "keep"
	}
	return d
}

// MoveToCold copies one log file to the cold directory. The
// caller is responsible for deleting the source after a
// successful copy.
func (m *Manager) MoveToCold(src, tenant string) (string, error) {
	if m.coldDir == "" {
		return "", errors.New("cold dir not configured")
	}
	if err := os.MkdirAll(filepath.Join(m.coldDir, tenant), 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(m.coldDir, tenant, fmt.Sprintf("%d-%s", m.now().UnixNano(), filepath.Base(src)))
	if err := copyFile(src, dst); err != nil {
		return "", err
	}
	info, err := os.Stat(dst)
	if err == nil {
		m.bytes += info.Size()
	}
	m.moved++
	return dst, nil
}

// SweepResult summarises one sweep run.
type SweepResult struct {
	Inspected int
	Dropped   int
	Moved     int
	Bytes     int64
}

// Sweep walks a slice of log rows and applies decisions in
// batch. The caller provides the input rows and a function
// that drops one row from the hot store.
func (m *Manager) Sweep(rows []Row, dropper func(Row) error, mover func(Row, string) error) (SweepResult, error) {
	res := SweepResult{Inspected: len(rows)}
	for _, r := range rows {
		ag := m.now().Sub(r.Timestamp)
		if ag < 0 {
			continue
		}
		d := m.Decide(r.Tenant, ag)
		switch d.Action {
		case "drop":
			if err := dropper(r); err != nil {
				return res, fmt.Errorf("drop: %w", err)
			}
			res.Dropped++
			m.sweeped++
		case "move_to_cold":
			dst := filepath.Join(m.coldDir, r.Tenant, fmt.Sprintf("%d-%s", r.Timestamp.UnixNano(), r.ID))
			if err := mover(r, dst); err != nil {
				return res, fmt.Errorf("move: %w", err)
			}
			res.Moved++
			m.moved++
		}
	}
	res.Bytes = m.bytes
	return res, nil
}

// Row is a generic log row that the sweeper acts on.
type Row struct {
	ID        string
	Tenant    string
	Timestamp time.Time
	Bytes     int64
}

// Stats is the JSON-stable view.
type Stats struct {
	Tenants   int  `json:"tenants"`
	Swept     int64 `json:"swept"`
	Moved     int64 `json:"moved"`
	Bytes     int64 `json:"bytes"`
	ColdDir   string `json:"cold_dir"`
}

// Stats returns current counters.
func (m *Manager) Stats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return Stats{
		Tenants: len(m.policies),
		Swept:   m.sweeped,
		Moved:   m.moved,
		Bytes:   m.bytes,
		ColdDir: m.coldDir,
	}
}

// RetentionReport describes what would happen on a sweep
// without modifying anything. Used by /debug/retention.
type RetentionReport struct {
	Tenant  string
	Tier    Tier
	Hot     time.Duration
	Cold    time.Duration
	Drop    int
	Move    int
	Keep    int
}

// Report inspects a snapshot of rows for one tenant.
func (m *Manager) Report(tenant string, rows []Row) RetentionReport {
	r := RetentionReport{Tenant: tenant}
	p, ok := m.Get(tenant)
	if !ok {
		r.Tier = TierFree
		r.Keep = len(rows)
		return r
	}
	r.Tier = p.Tier
	r.Hot = p.HotTTL
	r.Cold = p.ColdTTL
	for _, row := range rows {
		d := m.Decide(tenant, m.now().Sub(row.Timestamp))
		switch d.Action {
		case "drop":
			r.Drop++
		case "move_to_cold":
			r.Move++
		default:
			r.Keep++
		}
	}
	return r
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
