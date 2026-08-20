// Package grace 提供一个优雅停机协调器。
//
// 用法：
//
//	g := grace.New(30 * time.Second)
//	g.Register(grace.Hook{Name: "http", Fn: srv.Shutdown})
//	g.Register(grace.Hook{Name: "db", Fn: db.Close})
//	if err := g.Run(); err != nil { log.Println(err) }
//
// Shutdown 行为：
//   - 顺序按注册顺序执行所有 Hook
//   - 每个 Hook 在独立的 goroutine 中运行，受总 deadline 约束
//   - 某个 Hook 返回错误：记录并继续下一个（不中断）
//   - 超时：返回 ErrTimeout，剩余 Hook 不再等待（goroutine 仍在运行）
package grace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
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

// ErrShutdown 在重复调用 Shutdown 时返回。
var ErrShutdown = errors.New("grace: 重复 Shutdown")

// Manager 持有多个 Hook 并协调停机。
//
// 零值不可用；请使用 New。
type Manager struct {
	mu       sync.Mutex
	hooks    []Hook
	running  atomic.Bool // 防止并发 Shutdown
	done     atomic.Bool // Shutdown 已完成
	deadline time.Duration

	// 结果收集
	errs    []error
	elapsed atomic.Int64 // 纳秒

	onHookErr func(name string, err error)
}

// New 创建一个 Manager。
func New(deadline time.Duration) *Manager {
	if deadline <= 0 {
		deadline = 15 * time.Second
	}
	return &Manager{deadline: deadline}
}

// Register 注册一个 Hook。
// 注意：Shutdown 开始后不应再 Register。
func (m *Manager) Register(h Hook) {
	if h.Fn == nil {
		return
	}
	m.mu.Lock()
	m.hooks = append(m.hooks, h)
	m.mu.Unlock()
}

// RegisterWith 注册带单独超时名的 Hook（兼容 shutdownx 签名）。
// 单 hook 超时在内部通过 deadline 统一控制。
func (m *Manager) RegisterWith(name string, _ time.Duration, hook func(ctx context.Context) error) {
	m.Register(Hook{Name: name, Fn: hook})
}

// HookCount 返回已注册 Hook 数量。
func (m *Manager) HookCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.hooks)
}

// OnHookError 注册单 Hook 错误回调（不影响主流程）。
func (m *Manager) OnHookError(fn func(name string, err error)) {
	m.mu.Lock()
	m.onHookErr = fn
	m.mu.Unlock()
}

// WaitForSignal 阻塞直到收到 SIGINT/SIGTERM。
func (m *Manager) WaitForSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(ch)
	<-ch
}

// Shutdown 按注册顺序执行所有 Hook，受 deadline 约束。
// 所有 Hook 都会尝试执行；错误被收集。
// 总超时返回 ErrTimeout，同时仍返回所有已收集的错误。
func (m *Manager) Shutdown() error {
	if m.done.Load() {
		return ErrShutdown
	}
	if !m.running.CompareAndSwap(false, true) {
		// 并发调用 Shutdown：等待先驱者完成
		for m.running.Load() && !m.done.Load() {
			time.Sleep(10 * time.Millisecond)
		}
		return ErrShutdown
	}
	start := time.Now()
	defer func() {
		m.elapsed.Store(int64(time.Since(start)))
		m.done.Store(true)
	}()

	m.mu.Lock()
	hooks := append([]Hook(nil), m.hooks...)
	onHookErr := m.onHookErr
	m.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), m.deadline)
	defer cancel()

	var collected []error
	for _, h := range hooks {
		h := h
		done := make(chan error, 1)
		go func() {
			done <- safeRun(ctx, h.Fn)
		}()
		select {
		case <-ctx.Done():
			collected = append(collected, fmt.Errorf("%s: %w (hook 仍在运行)", h.Name, ErrTimeout))
			if onHookErr != nil {
				onHookErr(h.Name, ErrTimeout)
			}
			// 跳过剩余 hooks（但仍收集后续）
			continue
		case err := <-done:
			if err != nil {
				collected = append(collected, fmt.Errorf("%s: %w", h.Name, err))
				if onHookErr != nil {
					onHookErr(h.Name, err)
				}
			}
		}
	}

	m.mu.Lock()
	m.errs = collected
	m.mu.Unlock()

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return ErrTimeout
	}
	if len(collected) > 0 {
		return fmt.Errorf("grace: %d hook(s) failed: %v", len(collected), joinErrors(collected))
	}
	return nil
}

// Errors 返回所有 Hook 错误的副本（按发生顺序）。
func (m *Manager) Errors() []error {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]error, len(m.errs))
	copy(out, m.errs)
	return out
}

// Elapsed 返回 Shutdown 实际耗时。
func (m *Manager) Elapsed() time.Duration {
	return time.Duration(m.elapsed.Load())
}

// Run 阻塞等待信号并执行停机。
func (m *Manager) Run() error {
	m.WaitForSignal()
	return m.Shutdown()
}

// safeRun 在 panic 时返回错误。
func safeRun(ctx context.Context, fn func(context.Context) error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return fn(ctx)
}

// joinErrors 拼接多个错误为单行字符串。
func joinErrors(errs []error) string {
	if len(errs) == 0 {
		return ""
	}
	parts := make([]string, len(errs))
	for i, e := range errs {
		parts[i] = e.Error()
	}
	return strings.Join(parts, "; ")
}
