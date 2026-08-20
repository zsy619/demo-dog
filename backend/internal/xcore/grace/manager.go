package grace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// Manager 持有多个 Hook 并协调停机。
//
// 零值不可用；请使用 New。
//
// 线程安全：所有方法都可以并发调用；内部使用 sync.Mutex 保护 Hook 列表，
// 使用 atomic.Bool / atomic.Int64 保护运行标志、结果收集。
type Manager struct {
	mu       sync.Mutex // 保护 hooks / errs / onHookErr 的互斥锁
	hooks    []Hook     // 已注册的 Hook 列表（按顺序执行）
	running  atomic.Bool // 防止并发 Shutdown 的乐观锁
	done     atomic.Bool // Shutdown 已完成（用于快速拒绝重入）
	deadline time.Duration // 单次 Shutdown 的总截止时间

	// 结果收集
	errs    []error        // Shutdown 期间收集到的错误
	elapsed atomic.Int64  // 纳秒计的 Shutdown 耗时

	onHookErr func(name string, err error) // 单 Hook 错误回调（不影响主流程）
}

// New 创建一个 Manager。
//
// deadline <= 0 时使用默认 15 秒。
func New(deadline time.Duration) *Manager {
	if deadline <= 0 {
		deadline = 15 * time.Second
	}
	return &Manager{deadline: deadline}
}

// Register 注册一个 Hook。
//
// 注意：Shutdown 开始后不应再 Register。
// 当 h.Fn == nil 时静默忽略。
func (m *Manager) Register(h Hook) {
	if h.Fn == nil {
		return
	}
	m.mu.Lock()
	m.hooks = append(m.hooks, h)
	m.mu.Unlock()
}

// RegisterWith 注册带单独超时名的 Hook（兼容 shutdownx 签名）。
//
// 单 hook 超时在内部通过 deadline 统一控制；忽略传入的 _ time.Duration 参数。
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
//
// 当某个 Hook 返回错误时，会在收集错误之前先调用 fn(name, err)；
// fn 必须是非阻塞的，否则会拖慢 Shutdown。
func (m *Manager) OnHookError(fn func(name string, err error)) {
	m.mu.Lock()
	m.onHookErr = fn
	m.mu.Unlock()
}

// WaitForSignal 阻塞直到收到 SIGINT 或 SIGTERM 信号。
//
// 接收到信号后恢复默认信号处理行为（避免影响后续 Shutdown）。
func (m *Manager) WaitForSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(ch)
	<-ch
}

// Shutdown 按注册顺序执行所有 Hook，受 deadline 约束。
//
// 行为：
//  - 所有 Hook 都会尝试执行；错误被收集；
//  - 总超时时返回 ErrTimeout，同时仍返回所有已收集的错误；
//  - 重复调用 Shutdown 返回 ErrShutdown。
//
// 并发安全：使用 atomic.Bool 实现乐观锁，避免多个 goroutine 同时执行 Shutdown。
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
//
// 未执行过 Shutdown 时返回 0。
func (m *Manager) Elapsed() time.Duration {
	return time.Duration(m.elapsed.Load())
}

// Run 阻塞等待信号并执行停机。
//
// 典型场景用于 main() 中：
//
//	if err := mgr.Run(); err != nil { log.Println(err) }
func (m *Manager) Run() error {
	m.WaitForSignal()
	return m.Shutdown()
}
