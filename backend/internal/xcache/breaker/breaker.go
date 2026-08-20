// Package breaker 断路器：跟踪失败率，超过阈值时短路以保护下游。
package breaker

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// State is the breaker state.
type State int

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
		return "half-open"
	}
	return "unknown"
}

// Config describes the breaker thresholds.
type Config struct {
	Window        time.Duration
	MinSamples    int
	FailureRatio  float64
	OpenTimeout   time.Duration
	HalfOpenCalls int
}

// ErrOpen is returned when the breaker is open.
var ErrOpen = errors.New("breaker open")

// Breaker is an HTTP / RPC circuit breaker.
type Breaker struct {
	mu       sync.Mutex
	cfg      Config
	state    State
	calls    []outcome
	openAt   time.Time
	now      func() time.Time
	rejected atomic.Uint64
	succ     atomic.Uint64
	failed   atomic.Uint64
	shorts   atomic.Uint64
}

type outcome struct {
	success bool
	at      time.Time
}

// New creates a Breaker with the given config.
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

// WithTime overrides the time source for tests.
func (b *Breaker) WithTime(now func() time.Time) *Breaker {
	b.now = now
	return b
}

// State returns the current state.
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tickLocked()
	return b.state
}

// Allow checks whether the next call may proceed.
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

// Success records a successful call.
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

// Failure records a failed call.
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

func (b *Breaker) tickLocked() {
	if b.state == StateOpen && b.now().After(b.openAt.Add(b.cfg.OpenTimeout)) {
		b.state = StateHalfOpen
	}
}

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

// Stats returns counters.
type Stats struct {
	State    string `json:"state"`
	Accepted uint64 `json:"accepted"`
	Rejected uint64 `json:"rejected"`
	Success  uint64 `json:"success"`
	Failed   uint64 `json:"failed"`
	Shorts   uint64 `json:"shorts"`
}

// Stats returns the counter snapshot.
func (b *Breaker) Stats() Stats {
	b.mu.Lock()
	state := b.state
	b.mu.Unlock()
	return Stats{
		State:    state.String(),
		Accepted: b.succ.Load() + b.failed.Load(),
		Rejected: b.rejected.Load(),
		Success:  b.succ.Load(),
		Failed:   b.failed.Load(),
		Shorts:   b.shorts.Load(),
	}
}

// ShortCircuit increments the short-circuited counter and
// returns ErrOpen. Use when you want to record a rejection
// without calling Allow first.
func (b *Breaker) ShortCircuit() error {
	b.shorts.Add(1)
	return ErrOpen
}
