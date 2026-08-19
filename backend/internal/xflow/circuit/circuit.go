// Package circuit 提供一个可配置的断路器包装。
// 与 internal/breaker 类似，但参数化为三状态窗口：闭合/打开/半开。
package circuit

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// State 是断路器状态。
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

// ErrOpen 在断路器打开时被调用返回。
var ErrOpen = errors.New("circuit: 断路器打开")

// Config 描述策略参数。
type Config struct {
	FailureThreshold int
	OpenFor           time.Duration
	HalfOpenMax      int
}

// Default 返回默认配置。
func Default() Config {
	return Config{FailureThreshold: 5, OpenFor: 5 * time.Second, HalfOpenMax: 1}
}

// Breaker 是一个独立的三态断路器。
type Breaker struct {
	cfg          Config
	mu           sync.Mutex
	state        State
	failures     int
	openedAt     time.Time
	halfOpenDone atomic.Int32
	totalCalls   atomic.Uint64
	totalFailed  atomic.Uint64
	totalDenied  atomic.Uint64
}

// New 创建一个断路器。
func New(cfg Config) *Breaker {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 5
	}
	if cfg.OpenFor <= 0 {
		cfg.OpenFor = time.Second
	}
	if cfg.HalfOpenMax <= 0 {
		cfg.HalfOpenMax = 1
	}
	return &Breaker{cfg: cfg}
}

// Call 包装调用 fn，根据状态决定是否允许。
func (b *Breaker) Call(fn func() error) error {
	b.mu.Lock()
	switch b.state {
	case StateOpen:
		if time.Since(b.openedAt) >= b.cfg.OpenFor {
			b.state = StateHalfOpen
			b.halfOpenDone.Store(0)
		} else {
			b.mu.Unlock()
			b.totalDenied.Add(1)
			return ErrOpen
		}
	case StateHalfOpen:
		if b.halfOpenDone.Load() >= int32(b.cfg.HalfOpenMax) {
			b.mu.Unlock()
			b.totalDenied.Add(1)
			return ErrOpen
		}
		b.halfOpenDone.Add(1)
	}
	b.totalCalls.Add(1)
	b.mu.Unlock()
	err := fn()
	b.mu.Lock()
	if err != nil {
		b.failures++
		b.totalFailed.Add(1)
		if b.state == StateHalfOpen {
			b.state = StateOpen
			b.openedAt = time.Now()
		} else if b.failures >= b.cfg.FailureThreshold {
			b.state = StateOpen
			b.openedAt = time.Now()
		}
		b.mu.Unlock()
		return err
	}
	if b.state == StateHalfOpen {
		b.state = StateClosed
	}
	b.failures = 0
	b.mu.Unlock()
	return nil
}

// State 返回当前状态。
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// Stats 是计数器快照。
type Stats struct {
	Calls  uint64 `json:"calls"`
	Failed uint64 `json:"failed"`
	Denied uint64 `json:"denied"`
	State  State  `json:"state"`
}

// Stats 返回当前计数。
func (b *Breaker) Stats() Stats {
	b.mu.Lock()
	defer b.mu.Unlock()
	return Stats{Calls: b.totalCalls.Load(), Failed: b.totalFailed.Load(), Denied: b.totalDenied.Load(), State: b.state}
}

// Reset 强制重置为闭合状态。
func (b *Breaker) Reset() {
	b.mu.Lock()
	b.state = StateClosed
	b.failures = 0
	b.mu.Unlock()
}
