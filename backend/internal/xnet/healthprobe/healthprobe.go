// Package healthprobe 提供一个简单的 TCP 端口探活器：
// 周期性尝试连接一组主机，并标记健康状态。
package healthprobe

import (
	"context"
	"net"
	"sync"
	"time"
)

// Status 是主机的健康状态。
type Status int

const (
	StatusUnknown Status = iota
	StatusUp
	StatusDown
)

// Target 是一个探活目标。
type Target struct {
	Addr    string
	Status  Status
	Latency time.Duration
	LastAt  time.Time
}

// Prober 周期性地探活一组主机。
type Prober struct {
	mu       sync.RWMutex
	targets  map[string]*Target
	addrs    []string
	interval time.Duration
	timeout  time.Duration
	stop     chan struct{}
	running  bool
}

// New 创建一个 Prober。
func New(interval, timeout time.Duration) *Prober {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &Prober{
		targets:  make(map[string]*Target),
		interval: interval,
		timeout:  timeout,
		stop:     make(chan struct{}),
	}
}

// Add 加入一个目标地址。
func (p *Prober) Add(addr string) {
	p.mu.Lock()
	if _, ok := p.targets[addr]; !ok {
		p.targets[addr] = &Target{Addr: addr}
		p.addrs = append(p.addrs, addr)
	}
	p.mu.Unlock()
}

// Snapshot 返回当前所有目标的副本。
func (p *Prober) Snapshot() []Target {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]Target, 0, len(p.targets))
	for _, t := range p.targets {
		out = append(out, *t)
	}
	return out
}

// Status 返回单个目标的当前状态。
func (p *Prober) Status(addr string) Status {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if t, ok := p.targets[addr]; ok {
		return t.Status
	}
	return StatusUnknown
}

// Start 启动后台探活。
func (p *Prober) Start() {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	p.running = true
	p.mu.Unlock()
	go p.loop()
}

// Stop 停止后台探活。
func (p *Prober) Stop() {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	p.running = false
	p.mu.Unlock()
	close(p.stop)
}

func (p *Prober) loop() {
	t := time.NewTicker(p.interval)
	defer t.Stop()
	for {
		select {
		case <-p.stop:
			return
		case <-t.C:
			p.probeAll()
		}
	}
}

func (p *Prober) probeAll() {
	p.mu.RLock()
	addrs := make([]string, len(p.addrs))
	copy(addrs, p.addrs)
	p.mu.RUnlock()
	for _, a := range addrs {
		p.probeOne(a)
	}
}

func (p *Prober) probeOne(addr string) {
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	start := time.Now()
	d := net.Dialer{}
	c, err := d.DialContext(ctx, "tcp", addr)
	lat := time.Since(start)
	p.mu.Lock()
	t := p.targets[addr]
	if t == nil {
		p.mu.Unlock()
		return
	}
	t.LastAt = time.Now()
	t.Latency = lat
	if err != nil {
		t.Status = StatusDown
	} else {
		t.Status = StatusUp
		c.Close()
	}
	p.mu.Unlock()
}
