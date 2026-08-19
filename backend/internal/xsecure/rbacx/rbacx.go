// Package rbacx 提供轻量的 RBAC（基于角色的访问控制）引擎。
// 角色被授予一组权限；用户通过 Bind 关联一个或多个角色。
package rbacx

import "sync"

// Engine 是 RBAC 求值器。
type Engine struct {
	mu          sync.RWMutex
	roles       map[string]map[string]bool
	bindings    map[string]map[string]bool
}

// New 创建一个空 Engine。
func New() *Engine {
	return &Engine{
		roles:    make(map[string]map[string]bool),
		bindings: make(map[string]map[string]bool),
	}
}

// GrantRole 注册一个角色并赋予其权限。
func (e *Engine) GrantRole(role string, perms ...string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.roles[role] == nil {
		e.roles[role] = make(map[string]bool)
	}
	for _, p := range perms {
		e.roles[role][p] = true
	}
}

// Bind 把 user 绑定到 role。
func (e *Engine) Bind(user, role string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.bindings[user] == nil {
		e.bindings[user] = make(map[string]bool)
	}
	e.bindings[user][role] = true
}

// Unbind 解除 user 与 role 的关联。
func (e *Engine) Unbind(user, role string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if m := e.bindings[user]; m != nil {
		delete(m, role)
		if len(m) == 0 {
			delete(e.bindings, user)
		}
	}
}

// Allowed 返回 user 是否拥有 perm 权限。
func (e *Engine) Allowed(user, perm string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	roles, ok := e.bindings[user]
	if !ok {
		return false
	}
	for role := range roles {
		if e.roles[role][perm] {
			return true
		}
	}
	return false
}

// HasRole 返回 user 是否绑定 role。
func (e *Engine) HasRole(user, role string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, ok := e.bindings[user][role]
	return ok
}

// Roles 返回 user 的所有角色。
func (e *Engine) Roles(user string) []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := []string{}
	for r := range e.bindings[user] {
		out = append(out, r)
	}
	return out
}

// RolePerms 返回 role 的所有权限。
func (e *Engine) RolePerms(role string) []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := []string{}
	for p := range e.roles[role] {
		out = append(out, p)
	}
	return out
}

// Reset 清空所有映射。
func (e *Engine) Reset() {
	e.mu.Lock()
	e.roles = make(map[string]map[string]bool)
	e.bindings = make(map[string]map[string]bool)
	e.mu.Unlock()
}
