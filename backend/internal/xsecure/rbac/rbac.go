// Package rbac RBAC 引擎：角色继承 + 权限校验。
package rbac

import (
	"errors"
	"sync"
)

// Role is one named role with a permission set.
type Role struct {
	Name        string
	Permissions []string
	Parents     []string // roles whose permissions are inherited
}

// Assignment binds a subject to a role within a tenant.
type Assignment struct {
	Tenant  string
	Subject string
	Role    string
}

// Manager owns roles + per-tenant assignments.
type Manager struct {
	mu          sync.RWMutex
	roles       map[string]*Role
	assignments map[string]map[string]map[string]struct{} // tenant -> subject -> set of roles
}

// New creates an empty Manager.
func New() *Manager {
	return &Manager{
		roles:       make(map[string]*Role),
		assignments: make(map[string]map[string]map[string]struct{}),
	}
}

// ErrRoleExists is returned when Register is called twice.
var ErrRoleExists = errors.New("role already exists")

// ErrRoleMissing is returned when a referenced role does
// not exist.
var ErrRoleMissing = errors.New("role missing")

// Register adds a role.
func (m *Manager) Register(r *Role) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.roles[r.Name]; ok {
		return ErrRoleExists
	}
	for _, p := range r.Parents {
		if _, ok := m.roles[p]; !ok {
			return ErrRoleMissing
		}
	}
	m.roles[r.Name] = r
	return nil
}

// MustRegister panics on error.
func (m *Manager) MustRegister(r *Role) {
	if err := m.Register(r); err != nil {
		panic(err)
	}
}

// Get returns a role by name.
func (m *Manager) Get(name string) (*Role, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.roles[name]
	return r, ok
}

// Assign binds subject to role within tenant.
func (m *Manager) Assign(tenant, subject, role string) error {
	m.mu.Lock()
	if _, ok := m.roles[role]; !ok {
		m.mu.Unlock()
		return ErrRoleMissing
	}
	if _, ok := m.assignments[tenant]; !ok {
		m.assignments[tenant] = make(map[string]map[string]struct{})
	}
	if _, ok := m.assignments[tenant][subject]; !ok {
		m.assignments[tenant][subject] = make(map[string]struct{})
	}
	m.assignments[tenant][subject][role] = struct{}{}
	m.mu.Unlock()
	return nil
}

// Unassign removes the role from subject.
func (m *Manager) Unassign(tenant, subject, role string) {
	m.mu.Lock()
	if t, ok := m.assignments[tenant]; ok {
		if s, ok := t[subject]; ok {
			delete(s, role)
		}
	}
	m.mu.Unlock()
}

// Permission resolves the full set of permissions for
// subject within tenant, walking role inheritance.
func (m *Manager) Permission(tenant, subject, perm string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.hasPermLocked(tenant, subject, perm, make(map[string]bool))
}

func (m *Manager) hasPermLocked(tenant, subject, perm string, visited map[string]bool) bool {
	subs, ok := m.assignments[tenant]
	if !ok {
		return false
	}
	roles := subs[subject]
	for r := range roles {
		if visited[r] {
			continue
		}
		visited[r] = true
		role := m.roles[r]
		if role == nil {
			continue
		}
		for _, p := range role.Permissions {
			if p == perm {
				return true
			}
		}
		for _, parent := range role.Parents {
			if m.hasRolePermLocked(parent, perm, visited) {
				return true
			}
		}
	}
	return false
}

func (m *Manager) hasRolePermLocked(roleName, perm string, visited map[string]bool) bool {
	if visited[roleName] {
		return false
	}
	visited[roleName] = true
	role, ok := m.roles[roleName]
	if !ok {
		return false
	}
	for _, p := range role.Permissions {
		if p == perm {
			return true
		}
	}
	for _, parent := range role.Parents {
		if m.hasRolePermLocked(parent, perm, visited) {
			return true
		}
	}
	return false
}

// Roles returns the set of roles for a subject within a
// tenant (does not include parents).
func (m *Manager) Roles(tenant, subject string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	subs := m.assignments[tenant]
	if subs == nil {
		return nil
	}
	out := make([]string, 0)
	for r := range subs[subject] {
		out = append(out, r)
	}
	return out
}
