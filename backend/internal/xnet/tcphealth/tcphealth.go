// Package tcphealth TCP 健康探测：建立 TCP 连接并检测响应。
package tcphealth

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// Monitor tracks health of a list of TCP endpoints with
// adaptive timeouts.
type Monitor struct {
	mu        sync.RWMutex
	addrs     []string
	results   map[string]*Result
	timeout   time.Duration
	minDur    time.Duration
	maxDur    time.Duration
	attempted atomic.Uint64
	succeeded atomic.Uint64
	failed    atomic.Uint64
}

// Result is one endpoint's current state.
type Result struct {
	Addr      string
	Healthy   bool
	LastCheck time.Time
	Latency   time.Duration
	Err       error
}

// New creates a Monitor with the given targets and default
// timeout.
func New(addrs []string, timeout time.Duration) *Monitor {
	if timeout <= 0 {
		timeout = 500 * time.Millisecond
	}
	m := &Monitor{
		addrs:   append([]string{}, addrs...),
		results: make(map[string]*Result, len(addrs)),
		timeout: timeout,
		minDur:  10 * time.Millisecond,
		maxDur:  5 * time.Second,
	}
	for _, a := range addrs {
		m.results[a] = &Result{Addr: a}
	}
	return m
}

// Add registers a new endpoint.
func (m *Monitor) Add(addr string) {
	m.mu.Lock()
	m.addrs = append(m.addrs, addr)
	if _, ok := m.results[addr]; !ok {
		m.results[addr] = &Result{Addr: addr}
	}
	m.mu.Unlock()
}

// Check runs one health check round across all endpoints.
func (m *Monitor) Check(ctx context.Context) {
	m.mu.RLock()
	addrs := make([]string, len(m.addrs))
	copy(addrs, m.addrs)
	m.mu.RUnlock()
	for _, addr := range addrs {
		m.checkOne(ctx, addr)
	}
}

func (m *Monitor) checkOne(ctx context.Context, addr string) {
	m.mu.RLock()
	prev := m.results[addr]
	m.mu.RUnlock()
	to := m.timeout
	if prev.Latency > 0 {
		// Adaptive: scale timeout by 1.5x of last latency.
		to = prev.Latency * 3 / 2
		if to < m.minDur {
			to = m.minDur
		}
		if to > m.maxDur {
			to = m.maxDur
		}
	}
	cctx, cancel := context.WithTimeout(ctx, to)
	defer cancel()
	start := time.Now()
	var d net.Dialer
	conn, err := d.DialContext(cctx, "tcp", addr)
	latency := time.Since(start)
	m.attempted.Add(1)
	if err != nil {
		m.failed.Add(1)
		m.mu.Lock()
		prev.Healthy = false
		prev.LastCheck = time.Now()
		prev.Latency = latency
		prev.Err = err
		m.mu.Unlock()
		return
	}
	conn.Close()
	m.succeeded.Add(1)
	m.mu.Lock()
	prev.Healthy = true
	prev.LastCheck = time.Now()
	prev.Latency = latency
	prev.Err = nil
	m.mu.Unlock()
}

// Snapshot returns a copy of all results.
func (m *Monitor) Snapshot() map[string]Result {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]Result, len(m.results))
	for k, v := range m.results {
		out[k] = *v
	}
	return out
}

// ErrNoAddr is returned when no addresses are configured.
var ErrNoAddr = errors.New("no address")

// Healthiest returns the lowest-latency healthy endpoint.
func (m *Monitor) Healthiest() (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var best string
	var bestLat time.Duration
	first := true
	for _, r := range m.results {
		if !r.Healthy {
			continue
		}
		if first || r.Latency < bestLat {
			best = r.Addr
			bestLat = r.Latency
			first = false
		}
	}
	if best == "" {
		return "", ErrNoAddr
	}
	return best, nil
}

// Stats returns counters.
type Stats struct {
	Attempted uint64 `json:"attempted"`
	Succeeded uint64 `json:"succeeded"`
	Failed    uint64 `json:"failed"`
}

// Stats returns the snapshot.
func (m *Monitor) Stats() Stats {
	return Stats{Attempted: m.attempted.Load(), Succeeded: m.succeeded.Load(), Failed: m.failed.Load()}
}
