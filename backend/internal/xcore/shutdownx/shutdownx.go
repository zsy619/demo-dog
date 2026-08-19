// Package shutdownx 提供一个简单优雅停机协调器：
// 注册多个关闭 Hook，按 LIFO 顺序依次调用，每个 Hook 可设置超时。
package shutdownx

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Hook 是关闭时执行的回调。
type Hook func(ctx context.Context) error

// Manager 协调关闭流程。
type Manager struct {
	mu    sync.Mutex
	hooks []namedHook
}

type namedHook struct {
	name    string
	fn      Hook
	timeout time.Duration
}

// New 创建一个 Manager。
func New() *Manager {
	return &Manager{}
}

// Register 注册一个关闭 Hook；timeout 0 表示使用全局超时。
func (m *Manager) Register(name string, timeout time.Duration, fn Hook) {
	m.mu.Lock()
	m.hooks = append(m.hooks, namedHook{name: name, timeout: timeout, fn: fn})
	m.mu.Unlock()
}

// Shutdown 在 ctx 下按 LIFO 顺序执行所有 Hook。
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var errs []error
	for i := len(m.hooks) - 1; i >= 0; i-- {
		h := m.hooks[i]
		c := ctx
		if h.timeout > 0 {
			var cancel context.CancelFunc
			c, cancel = context.WithTimeout(ctx, h.timeout)
			defer cancel()
		}
		if err := h.fn(c); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", h.name, err))
		}
	}
	m.hooks = nil
	return errors.Join(errs...)
}

// HookCount 返回当前 Hook 数。
func (m *Manager) HookCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.hooks)
}
