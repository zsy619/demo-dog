// Package grace 提供一个优雅停机协调器。
// 它按注册顺序关闭 Hooks，并在指定总超时内尽力完成所有清理。
package grace

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// Hook 表示一项关闭动作。
type Hook struct {
	Name string
	Fn   func(ctx context.Context) error
}

// ErrTimeout 在总超时时返回。
var ErrTimeout = errors.New("grace: 停机超时")

// Manager 持有多个 Hook 并协调停机。
type Manager struct {
	mu       sync.Mutex
	hooks    []Hook
	closed   atomic.Bool
	deadline time.Duration
}

// New 创建一个 Manager。
func New(deadline time.Duration) *Manager {
	if deadline <= 0 {
		deadline = 15 * time.Second
	}
	return &Manager{deadline: deadline}
}

// Register 注册一个 Hook。
func (m *Manager) Register(h Hook) {
	m.mu.Lock()
	m.hooks = append(m.hooks, h)
	m.mu.Unlock()
}

// WaitForSignal 阻塞直到收到 SIGINT/SIGTERM。
func (m *Manager) WaitForSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
}

// Shutdown 按注册顺序执行所有 Hook，受 deadline 约束。
// 一旦超时就返回 ErrTimeout，未完成的 Hook 不再被等待。
func (m *Manager) Shutdown() error {
	if !m.closed.CompareAndSwap(false, true) {
		return nil
	}
	m.mu.Lock()
	hooks := append([]Hook(nil), m.hooks...)
	m.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), m.deadline)
	defer cancel()
	for _, h := range hooks {
		h := h
		done := make(chan error, 1)
		go func() {
			done <- safeRun(h.Fn)
		}()
		select {
		case <-ctx.Done():
			return ErrTimeout
		case err := <-done:
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Run 阻塞等待信号并执行停机。
func (m *Manager) Run() error {
	m.WaitForSignal()
	return m.Shutdown()
}

func safeRun(fn func(context.Context) error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New("panic")
		}
	}()
	return fn(context.Background())
}

// HookCount 返回已注册 Hook 数量。
func (m *Manager) HookCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.hooks)
}
