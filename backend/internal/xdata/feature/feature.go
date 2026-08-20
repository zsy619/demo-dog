package feature

// Per-tenant feature flags with audit trail.
//
// Until now operators had no way to gate a beta feature for a
// single tenant. With Round 56 the flag table is per-tenant,
// every change appends an audit entry (who, when, before,
// after), and the Evaluate helper falls back to a global
// default if the tenant has no override.
//
// Flags are typed: bool, string, int. The Evaluate function
// returns the per-tenant value or the global default.

import (
	"errors"
	"sort"
	"sync"
	"time"
)

// Kind identifies the value type of a flag.
type Kind int

const (
	KindBool Kind = iota
	KindString
	KindInt
)

// Flag is the metadata for one named feature.
type Flag struct {
	Name        string
	Description string
	Kind        Kind
	Default     any
}

// Validate runs basic integrity checks on the flag.
func (f *Flag) Validate() error {
	if f.Name == "" {
		return errors.New("flag name required")
	}
	if f.Kind == KindBool {
		if _, ok := f.Default.(bool); !ok {
			return errors.New("bool flag default must be bool")
		}
	}
	if f.Kind == KindString {
		if _, ok := f.Default.(string); !ok {
			return errors.New("string flag default must be string")
		}
	}
	if f.Kind == KindInt {
		if _, ok := f.Default.(int); !ok {
			return errors.New("int flag default must be int")
		}
	}
	return nil
}

// Override is one tenant-specific value.
type Override struct {
	Tenant    string
	Value     any
	UpdatedAt time.Time
	UpdatedBy string
}

// AuditEntry records one change.
type AuditEntry struct {
	Tenant     string
	Flag       string
	OldValue   any
	NewValue   any
	Actor      string
	At         time.Time
	Action     string // set, clear
}

// Manager is the flag table.
type Manager struct {
	mu       sync.RWMutex
	flags    map[string]*Flag
	over     map[string]map[string]*Override // flag -> tenant -> override
	audit    []AuditEntry
	auditCap int
	now      func() time.Time
}

// NewManager returns an empty manager.
func NewManager(auditCap int) *Manager {
	if auditCap <= 0 {
		auditCap = 1024
	}
	return &Manager{
		flags:    make(map[string]*Flag),
		over:     make(map[string]map[string]*Override),
		auditCap: auditCap,
		now:      time.Now,
	}
}

// WithTime overrides the time source.
func (m *Manager) WithTime(now func() time.Time) *Manager {
	m.now = now
	return m
}

// Register adds a flag definition.
func (m *Manager) Register(f *Flag) error {
	if err := f.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.flags[f.Name]; ok {
		return errors.New("flag already registered")
	}
	m.flags[f.Name] = f
	if _, ok := m.over[f.Name]; !ok {
		m.over[f.Name] = make(map[string]*Override)
	}
	return nil
}

// MustRegister panics on error.
func (m *Manager) MustRegister(f *Flag) {
	if err := m.Register(f); err != nil {
		panic(err)
	}
}

// Get returns the flag definition.
func (m *Manager) Get(name string) (*Flag, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	f, ok := m.flags[name]
	return f, ok
}

// List returns all flag names sorted.
func (m *Manager) List() []*Flag {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Flag, 0, len(m.flags))
	for _, f := range m.flags {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Evaluate returns the effective value of flag for tenant.
func (m *Manager) Evaluate(name, tenant string) (any, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	f, ok := m.flags[name]
	if !ok {
		return nil, false
	}
	if o, ok := m.over[name][tenant]; ok {
		return o.Value, true
	}
	return f.Default, true
}

// Bool / String / Int typed accessors. Each returns the
// default if no override is set.
func (m *Manager) Bool(name, tenant string) (bool, bool) {
	v, ok := m.Evaluate(name, tenant)
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

// String returns the string value for the tenant.
func (m *Manager) String(name, tenant string) (string, bool) {
	v, ok := m.Evaluate(name, tenant)
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// Int returns the int value for the tenant.
func (m *Manager) Int(name, tenant string) (int, bool) {
	v, ok := m.Evaluate(name, tenant)
	if !ok {
		return 0, false
	}
	i, ok := v.(int)
	return i, ok
}

// SetOverride sets the per-tenant override and records audit.
func (m *Manager) SetOverride(name, tenant string, value any, actor string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.flags[name]
	if !ok {
		return errors.New("flag not registered")
	}
	if err := kindMatches(f, value); err != nil {
		return err
	}
	old := m.over[name][tenant]
	var oldVal any
	if old != nil {
		oldVal = old.Value
	}
	m.over[name][tenant] = &Override{
		Tenant: tenant, Value: value,
		UpdatedAt: m.now(), UpdatedBy: actor,
	}
	m.recordAudit(AuditEntry{
		Tenant: tenant, Flag: name,
		OldValue: oldVal, NewValue: value,
		Actor: actor, At: m.now(), Action: "set",
	})
	return nil
}

// ClearOverride removes a tenant override and reverts to the
// default.
func (m *Manager) ClearOverride(name, tenant, actor string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.flags[name]; !ok {
		return errors.New("flag not registered")
	}
	old, ok := m.over[name][tenant]
	if !ok {
		return nil
	}
	var oldVal any
	if old != nil {
		oldVal = old.Value
	}
	delete(m.over[name], tenant)
	m.recordAudit(AuditEntry{
		Tenant: tenant, Flag: name,
		OldValue: oldVal, NewValue: nil,
		Actor: actor, At: m.now(), Action: "clear",
	})
	return nil
}

// Overrides returns all overrides for one flag.
func (m *Manager) Overrides(name string) []*Override {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tenants := m.over[name]
	out := make([]*Override, 0, len(tenants))
	for _, o := range tenants {
		out = append(out, o)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Tenant < out[j].Tenant })
	return out
}

// Audit returns a copy of the audit ring (most recent last).
func (m *Manager) Audit() []AuditEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]AuditEntry, len(m.audit))
	copy(out, m.audit)
	return out
}

// AuditFor returns audit entries scoped to one tenant.
func (m *Manager) AuditFor(tenant string) []AuditEntry {
	all := m.Audit()
	out := make([]AuditEntry, 0, len(all))
	for _, a := range all {
		if a.Tenant == tenant {
			out = append(out, a)
		}
	}
	return out
}

func (m *Manager) recordAudit(e AuditEntry) {
	if len(m.audit) >= m.auditCap {
		m.audit = m.audit[1:]
	}
	m.audit = append(m.audit, e)
}

func kindMatches(f *Flag, v any) error {
	switch f.Kind {
	case KindBool:
		if _, ok := v.(bool); !ok {
			return errors.New("bool flag requires bool value")
		}
	case KindString:
		if _, ok := v.(string); !ok {
			return errors.New("string flag requires string value")
		}
	case KindInt:
		if _, ok := v.(int); !ok {
			return errors.New("int flag requires int value")
		}
	}
	return nil
}
