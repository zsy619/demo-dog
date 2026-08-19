// Package routex 提供一个简单的最长前缀匹配路由表（用于网关分流）。
package routex

import (
	"sort"
	"strings"
	"sync"
)

// Route 表示一个前缀路由。
type Route struct {
	Prefix string
	Target string
}

// Table 是最长前缀匹配的路由表。
type Table struct {
	mu     sync.RWMutex
	routes []Route
}

// New 创建一个空 Table。
func New() *Table { return &Table{} }

// Add 添加一个路由。
func (t *Table) Add(prefix, target string) {
	t.mu.Lock()
	t.routes = append(t.routes, Route{prefix, target})
	t.mu.Unlock()
}

// Match 查找最长前缀匹配。
func (t *Table) Match(p string) (string, bool) {
	t.mu.RLock()
	routes := make([]Route, len(t.routes))
	copy(routes, t.routes)
	t.mu.RUnlock()
	sort.Slice(routes, func(i, j int) bool {
		return len(routes[i].Prefix) > len(routes[j].Prefix)
	})
	for _, r := range routes {
		if strings.HasPrefix(p, r.Prefix) {
			return r.Target, true
		}
	}
	return "", false
}

// Clear 清空路由。
func (t *Table) Clear() {
	t.mu.Lock()
	t.routes = nil
	t.mu.Unlock()
}

// Len 返回路由数。
func (t *Table) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.routes)
}
