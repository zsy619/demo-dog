// Package keepalive 提供 keepalive 检测：定期 ping，超时视为失败。
package keepalive

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Pinger 是被监控的对象，需实现 Ping 方法。
type Pinger interface {
	Ping() error
}

// Monitor 是 keepalive 监控器。
type Monitor struct {
	interval time.Duration
	timeout  time.Duration
	target   Pinger
	mu       sync.Mutex
	stop     chan struct{}
	running  atomic.Bool
	misses   atomic.Int32
	successes atomic.Int64
}

// New 创建一个 Monitor。
func New(target Pinger, interval, timeout time.Duration) *Monitor {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if timeout <= 0 {
		timeout = interval
	}
	return &Monitor{target: target, interval: interval, timeout: timeout, stop: make(chan struct{})}
}

// Start 启动监控 goroutine。
func (m *Monitor) Start(ctx context.Context) {
	if !m.running.CompareAndSwap(false, true) {
		return
	}
	go m.loop(ctx)
}

// Stop 停止监控。
func (m *Monitor) Stop() {
	if !m.running.CompareAndSwap(true, false) {
		return
	}
	close(m.stop)
}

// Misses 返回累计失败次数。
func (m *Monitor) Misses() int32 { return m.misses.Load() }

// Successes 返回累计成功次数。
func (m *Monitor) Successes() int64 { return m.successes.Load() }

func (m *Monitor) loop(ctx context.Context) {
	t := time.NewTicker(m.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stop:
			return
		case <-t.C:
			m.tick(ctx)
		}
	}
}

func (m *Monitor) tick(ctx context.Context) {
	cctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- m.target.Ping() }()
	select {
	case err := <-done:
		if err != nil {
			m.misses.Add(1)
		} else {
			m.successes.Add(1)
		}
	case <-cctx.Done():
		m.misses.Add(1)
	}
}
