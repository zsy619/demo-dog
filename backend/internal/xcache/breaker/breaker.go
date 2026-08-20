package breaker

// breaker.go：Breaker 主体实现。
//
// 包含状态机驱动、滑动窗口、阈值评估等核心逻辑；
// 所有公共方法都使用 mu 保护状态字段；计数器字段使用 atomic。

import (
	"sync"
	"sync/atomic"
	"time"
)

// Breaker 是 HTTP / RPC 等远程调用场景的断路器。
//
// 工作流程：
//  1. 调用方在执行远程调用前先调 Allow()，若返回 true 则继续；
//  2. 无论调用成功还是失败，都应分别调 Success() 或 Failure() 记录结果；
//  3. 当滑动窗口内失败率达到 FailureRatio 且样本数 ≥ MinSamples 时进入 StateOpen；
//  4. StateOpen 持续 OpenTimeout 后转入 StateHalfOpen；
//  5. StateHalfOpen 期间通过的调用若成功则回到 StateClosed；
//     若失败则重新进入 StateOpen。
//
// 线程安全：所有状态字段由 mu 保护；计数器字段使用 atomic.Load/Add。
type Breaker struct {
	mu       sync.Mutex     // 保护 state / calls / openAt / now 的互斥锁
	cfg      Config         // 阈值配置
	state    State          // 当前状态机
	calls    []outcome      // 滑动窗口内的样本
	openAt   time.Time      // 进入 Open 状态的时间戳
	now      func() time.Time // 时间源（便于测试注入）
	rejected atomic.Uint64  // Allow 返回 false 的次数
	succ     atomic.Uint64  // Success 累计调用次数
	failed   atomic.Uint64  // Failure 累计调用次数
	shorts   atomic.Uint64  // ShortCircuit 累计调用次数
}

// New 创建并初始化一个 Breaker。
//
// 当 cfg 中某个字段为零或负时，自动填充默认值：
//   - Window:        10 * time.Second
//   - MinSamples:    5
//   - FailureRatio:  0.5
//   - OpenTimeout:   30 * time.Second
//   - HalfOpenCalls: 1
func New(cfg Config) *Breaker {
	if cfg.Window <= 0 {
		cfg.Window = 10 * time.Second
	}
	if cfg.MinSamples <= 0 {
		cfg.MinSamples = 5
	}
	if cfg.FailureRatio <= 0 {
		cfg.FailureRatio = 0.5
	}
	if cfg.OpenTimeout <= 0 {
		cfg.OpenTimeout = 30 * time.Second
	}
	if cfg.HalfOpenCalls <= 0 {
		cfg.HalfOpenCalls = 1
	}
	return &Breaker{cfg: cfg, state: StateClosed, now: time.Now}
}

// WithTime 注入自定义时间源（仅供测试使用）。
//
// 返回 *Breaker 以支持链式调用：b := breaker.New(cfg).WithTime(fakeNow)。
func (b *Breaker) WithTime(now func() time.Time) *Breaker {
	b.now = now
	return b
}

// State 返回当前状态（同时执行 tickLocked 检查是否需要从 Open 转 HalfOpen）。
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tickLocked()
	return b.state
}

// Allow 判断下一次调用是否可以放行。
//
// 规则：
//   - StateClosed / StateHalfOpen: 返回 true；
//   - StateOpen: rejected +1，返回 false。
//
// 调用方在收到 true 后无论成功失败都应调 Success / Failure 记录结果。
func (b *Breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tickLocked()
	switch b.state {
	case StateClosed, StateHalfOpen:
		return true
	case StateOpen:
		b.rejected.Add(1)
		return false
	}
	return false
}

// Success 记录一次成功的调用。
//
// 在 StateHalfOpen 时，成功调用会让状态回到 StateClosed 并清空样本；
// 在 StateClosed 时仅追加样本到滑动窗口。
func (b *Breaker) Success() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls = append(b.calls, outcome{success: true, at: b.now()})
	b.succ.Add(1)
	b.trimLocked()
	if b.state == StateHalfOpen {
		b.state = StateClosed
		b.calls = nil
	}
}

// Failure 记录一次失败的调用。
//
// 在 StateHalfOpen 时任何一次失败都会立即重新进入 StateOpen；
// 在 StateClosed 时会检查窗口内失败率，超过阈值则熔断。
func (b *Breaker) Failure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls = append(b.calls, outcome{success: false, at: b.now()})
	b.failed.Add(1)
	b.trimLocked()
	if b.state == StateHalfOpen {
		b.state = StateOpen
		b.openAt = b.now()
		return
	}
	if len(b.calls) >= b.cfg.MinSamples {
		ratio := b.failureRatioLocked()
		if ratio >= b.cfg.FailureRatio {
			b.state = StateOpen
			b.openAt = b.now()
		}
	}
}

// tickLocked 检查 Open 超时，必要时转 HalfOpen。
//
// 必须在持锁状态下调用。
func (b *Breaker) tickLocked() {
	if b.state == StateOpen && b.now().After(b.openAt.Add(b.cfg.OpenTimeout)) {
		b.state = StateHalfOpen
	}
}

// trimLocked 修剪滑动窗口，丢弃超出 Window 时长范围的旧样本。
//
// 必须在持锁状态下调用；原地复用 b.calls 切片以减少内存分配。
func (b *Breaker) trimLocked() {
	cutoff := b.now().Add(-b.cfg.Window)
	out := b.calls[:0]
	for _, c := range b.calls {
		if c.at.After(cutoff) {
			out = append(out, c)
		}
	}
	b.calls = out
}

// failureRatioLocked 计算窗口内失败率（0 表示无样本）。
//
// 必须在持锁状态下调用。
func (b *Breaker) failureRatioLocked() float64 {
	if len(b.calls) == 0 {
		return 0
	}
	fails := 0
	for _, c := range b.calls {
		if !c.success {
			fails++
		}
	}
	return float64(fails) / float64(len(b.calls))
}
