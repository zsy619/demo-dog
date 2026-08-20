// Package rbac RBAC 引擎：角色继承 + 权限校验。
package rbac

import (
	"errors"
	"sync"
)

// Role 是带有一组权限的单一命名角色。
type Role struct {
	Name        string
	Permissions []string
	Parents     []string // roles whose permissions are inherited
}

// Assignment 将主体绑定到租户内的某个角色。
type Assignment struct {
	Tenant  string
	Subject string
	Role    string
}

// Manager 持有角色以及每个租户的分配关系。
type Manager struct {
	mu          sync.RWMutex
	roles       map[string]*Role
	assignments map[string]map[string]map[string]struct{} // tenant -> subject -> set of roles
}

// New 创建一个空的 Manager。
func New() *Manager {
	return &Manager{
		roles:       make(map[string]*Role),
		assignments: make(map[string]map[string]map[string]struct{}),
	}
}

// ErrRoleExists 在 Register 被重复调用时返回。
var ErrRoleExists = errors.New("role already exists")

// ErrRoleMissing 在引用的 role 不存在时返回。
// not exist.
var ErrRoleMissing = errors.New("role missing")

// Register 添加一个角色。
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

// MustRegister 在出错时 panic。
func (m *Manager) MustRegister(r *Role) {
	if err := m.Register(r); err != nil {
		panic(err)
	}
}

// Get 按名称返回一个角色。
func (m *Manager) Get(name string) (*Role, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.roles[name]
	return r, ok
}

// Assign 将主体绑定到租户内的某个角色。
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

// Unassign 从主体上移除该角色。
func (m *Manager) Unassign(tenant, subject, role string) {
	m.mu.Lock()
	if t, ok := m.assignments[tenant]; ok {
		if s, ok := t[subject]; ok {
			delete(s, role)
		}
	}
	m.mu.Unlock()
}

// Permission 在 tenant 内解析 subject 的完整权限集合，遍历 role 继承关系。
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

// Roles 返回一个 tenant 内 subject 的 role 集合（不包括父 role）。
// Roles 返回一个 tenant 内 subject 的 role 集合（不包括父 role）。
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
