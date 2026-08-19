// Package clockx 提供可注入的单调时钟抽象，便于测试。
package clockx

import (
	"sync"
	"time"
)

// Clock 是一个可被替换的时钟接口。
type Clock interface {
	Now() time.Time
	Since(t time.Time) time.Duration
	Sleep(d time.Duration)
	NewTicker(d time.Duration) Ticker
}

// Ticker 是 time.Ticker 的抽象。
type Ticker interface {
	C() <-chan time.Time
	Stop()
}

// Real 是真实时钟。
type Real struct{}

// Now 返回当前时间。
func (Real) Now() time.Time { return time.Now() }

// Since 返回 t 到现在的时长。
func (c Real) Since(t time.Time) time.Duration { return time.Since(t) }

// Sleep 睡眠 d。
func (c Real) Sleep(d time.Duration) { time.Sleep(d) }

// NewTicker 返回真实 Ticker。
func (c Real) NewTicker(d time.Duration) Ticker { return &realTicker{t: time.NewTicker(d)} }

type realTicker struct {
	t *time.Ticker
}

func (r *realTicker) C() <-chan time.Time { return r.t.C }

func (r *realTicker) Stop() { r.t.Stop() }

// Fake 是可由测试控制的虚拟时钟。
type Fake struct {
	mu  sync.Mutex
	cur time.Time
}

// NewFake 从 t 开始构造 Fake。
func NewFake(t time.Time) *Fake { return &Fake{cur: t} }

// Now 返回当前虚拟时间。
func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cur
}

// Since 返回从 t 到当前虚拟时间的时长。
func (f *Fake) Since(t time.Time) time.Duration {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cur.Sub(t)
}

// Sleep 立即推进 d（不真正等待）。
func (f *Fake) Sleep(d time.Duration) {
	f.mu.Lock()
	f.cur = f.cur.Add(d)
	f.mu.Unlock()
}

// Advance 推进虚拟时间。
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	f.cur = f.cur.Add(d)
	f.mu.Unlock()
}

// NewTicker 返回一个 Fake ticker（不实现）。
func (f *Fake) NewTicker(d time.Duration) Ticker { return &fakeTicker{f: f, d: d} }

type fakeTicker struct {
	f *Fake
	d time.Duration
}

func (t *fakeTicker) C() <-chan time.Time {
	ch := make(chan time.Time)
	go func() {
		t.f.mu.Lock()
		at := t.f.cur.Add(t.d)
		t.f.mu.Unlock()
		ch <- at
	}()
	return ch
}

func (t *fakeTicker) Stop() {}
