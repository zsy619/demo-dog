// Package circuit provides a small circuit-breaker.
package circuit

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// State is the breaker state.
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

// ErrOpen is returned when the breaker is open and rejected
// the call.
var ErrOpen = errors.New("circuit breaker is open")

// Settings configures the breaker.
type Settings struct {
	FailureThreshold int
	CoolDown         time.Duration
	Now              func() time.Time
}

// Breaker is a single circuit breaker.
type Breaker struct {
	settings Settings
	mu       sync.Mutex
	state    atomic.Int32
	fails    atomic.Int64
	openedAt atomic.Int64
}

// New constructs a Breaker.
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

// State returns the current state.
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

// Allow returns nil if the call should proceed.
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

// Success records a successful call.
func (b *Breaker) Success() {
	s := State(b.state.Load())
	if s == StateHalfOpen || s == StateOpen {
		b.mu.Lock()
		b.state.Store(int32(StateClosed))
		b.mu.Unlock()
	}
	b.fails.Store(0)
}

// Failure records a failed call.
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

// Snapshot is the JSON-stable view.
type Snapshot struct {
	State         string `json:"state"`
	Failures      int64  `json:"failures"`
	Threshold     int    `json:"threshold"`
	CoolDownNanos int64  `json:"cool_down_ns"`
	OpenedAt      string `json:"opened_at,omitempty"`
}

// Snapshot returns the current breaker view.
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
