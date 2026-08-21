// Package circuit 提供了一个轻量的断路器实现。
package circuit

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// State 表示断路器的状态。
type State int32

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half_open"
	}
	return "unknown"
}

// ErrOpen 在断路器处于 Open 状态并拒绝调用时返回。
var ErrOpen = errors.New("circuit breaker is open")

// Settings 用于配置断路器。
type Settings struct {
	FailureThreshold int
	CoolDown         time.Duration
	Now              func() time.Time
}

// Breaker 是一个独立的断路器实例。
type Breaker struct {
	settings Settings
	mu       sync.Mutex
	state    atomic.Int32
	fails    atomic.Int64
	openedAt atomic.Int64
}

// New 构造一个 Breaker。
func New(s Settings) *Breaker {
	if s.FailureThreshold <= 0 {
		s.FailureThreshold = 5
	}
	if s.CoolDown <= 0 {
		s.CoolDown = 30 * time.Second
	}
	if s.Now == nil {
		s.Now = time.Now
	}
	return &Breaker{settings: s}
}

// State 返回当前状态。
func (b *Breaker) State() State {
	s := State(b.state.Load())
	if s == StateOpen {
		opened := b.openedAt.Load()
		if b.settings.Now().UnixNano()-opened >= int64(b.settings.CoolDown) {
			b.mu.Lock()
			if State(b.state.Load()) == StateOpen {
				b.state.Store(int32(StateHalfOpen))
				s = StateHalfOpen
			}
			b.mu.Unlock()
		}
	}
	return s
}

// Allow 在调用应继续时返回 nil。
func (b *Breaker) Allow() error {
	switch b.State() {
	case StateClosed:
		return nil
	case StateOpen:
		return ErrOpen
	case StateHalfOpen:
		b.mu.Lock()
		defer b.mu.Unlock()
		if State(b.state.Load()) == StateHalfOpen {
			b.state.Store(int32(StateOpen))
			b.openedAt.Store(b.settings.Now().UnixNano())
			return nil
		}
		return ErrOpen
	}
	return ErrOpen
}

// Success 记录一次成功的调用。
func (b *Breaker) Success() {
	s := State(b.state.Load())
	if s == StateHalfOpen || s == StateOpen {
		b.mu.Lock()
		b.state.Store(int32(StateClosed))
		b.mu.Unlock()
	}
	b.fails.Store(0)
}

// Failure 记录一次失败的调用。
func (b *Breaker) Failure() {
	n := b.fails.Add(1)
	if n >= int64(b.settings.FailureThreshold) {
		b.mu.Lock()
		if State(b.state.Load()) == StateClosed {
			b.state.Store(int32(StateOpen))
			b.openedAt.Store(b.settings.Now().UnixNano())
		}
		b.mu.Unlock()
	}
}

// Snapshot 是 JSON 稳定的视图。
type Snapshot struct {
	State         string `json:"state"`
	Failures      int64  `json:"failures"`
	Threshold     int    `json:"threshold"`
	CoolDownNanos int64  `json:"cool_down_ns"`
	OpenedAt      string `json:"opened_at,omitempty"`
}

// Snapshot 返回当前断路器的视图。
func (b *Breaker) Snapshot() Snapshot {
	snap := Snapshot{
		State:         b.State().String(),
		Failures:      b.fails.Load(),
		Threshold:     b.settings.FailureThreshold,
		CoolDownNanos: int64(b.settings.CoolDown),
	}
	if o := b.openedAt.Load(); o > 0 {
		snap.OpenedAt = time.Unix(0, o).UTC().Format(time.RFC3339Nano)
	}
	return snap
}

// Restore 把一个快照强制写回到断路器。用于
// 进程重启后恢复 breaker 状态(例如刚刚把 ingest
// 标记为 open,我们重启后不希望它忘记这件事)。
//
// 必须持锁外部调用方;该函数内部不锁。
func (b *Breaker) Restore(snap Snapshot) {
	switch snap.State {
	case "open":
		b.state.Store(int32(StateOpen))
	case "half_open":
		b.state.Store(int32(StateHalfOpen))
	default:
		b.state.Store(int32(StateClosed))
	}
	if snap.Failures > 0 {
		b.fails.Store(snap.Failures)
	} else {
		b.fails.Store(0)
	}
	if t, err := time.Parse(time.RFC3339Nano, snap.OpenedAt); err == nil && !t.IsZero() {
		b.openedAt.Store(t.UnixNano())
	} else if snap.State != "open" {
		b.openedAt.Store(0)
	}
}
