// Package probe 提供主动健康探测：
// 定期对一组目标执行探测，按状态维度输出结构化结果。
package probe

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// Status 表示探测结果。
type Status int

const (
	StatusUnknown Status = iota
	StatusHealthy
	StatusDegraded
	StatusUnhealthy
)

// Probe 是单个目标的探测函数。
type Probe func(ctx context.Context, target string) error

// Target 描述一个被探测对象。
type Target struct {
	Name string
	Addr string
}

// Result 是单次探测结果。
type Result struct {
	Target  string        `json:"target"`
	Status  Status        `json:"status"`
	Latency time.Duration `json:"latency"`
	Error   string        `json:"error,omitempty"`
	At      time.Time     `json:"at"`
}

// ErrEmptyTargets 在目标列表为空时返回。
var ErrEmptyTargets = errors.New("probe: 目标列表为空")

// Prober 调度一组目标的周期性探测。
type Prober struct {
	mu       sync.Mutex
	interval time.Duration
	timeout  time.Duration
	probe    Probe
	targets  []Target
	results  sync.Map
	stop     chan struct{}
	run      atomic.Bool
	round    atomic.Uint64
}

// Config 描述参数。
type Config struct {
	Interval time.Duration
	Timeout  time.Duration
}

// New 创建一个 Prober。
func New(cfg Config, p Probe, targets []Target) *Prober {
	if cfg.Interval <= 0 {
		cfg.Interval = 5 * time.Second
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 2 * time.Second
	}
	if p == nil {
		p = func(_ context.Context, _ string) error { return nil }
	}
	return &Prober{
		interval: cfg.Interval,
		timeout:  cfg.Timeout,
		probe:    p,
		targets:  targets,
		stop:     make(chan struct{}),
	}
}

// Start 启动探测循环。
func (p *Prober) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.run.CompareAndSwap(false, true) {
		return
	}
	go p.loop()
}

// Stop 停止探测。
func (p *Prober) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.run.CompareAndSwap(true, false) {
		return
	}
	close(p.stop)
	p.stop = make(chan struct{})
}

// Snapshot 返回所有目标的最新结果。
func (p *Prober) Snapshot() []Result {
	out := []Result{}
	p.results.Range(func(k, v any) bool {
		out = append(out, v.(Result))
		return true
	})
	return out
}

// ResultOf 返回单个目标的最新结果。
func (p *Prober) ResultOf(name string) (Result, bool) {
	v, ok := p.results.Load(name)
	if !ok {
		return Result{}, false
	}
	return v.(Result), true
}

// RunOnce 执行一轮所有目标探测。
func (p *Prober) RunOnce(ctx context.Context) error {
	if len(p.targets) == 0 {
		return ErrEmptyTargets
	}
	p.round.Add(1)
	for _, t := range p.targets {
		t := t
		c, c2 := context.WithTimeout(ctx, p.timeout)
		start := time.Now()
		err := p.probe(c, t.Addr)
		c2()
		res := Result{Target: t.Name, Latency: time.Since(start), At: time.Now()}
		if err == nil {
			res.Status = StatusHealthy
		} else {
			res.Status = StatusUnhealthy
			res.Error = err.Error()
		}
		p.results.Store(t.Name, res)
	}
	return nil
}

func (p *Prober) loop() {
	t := time.NewTicker(p.interval)
	defer t.Stop()
	for {
		p.mu.Lock()
		stop := p.stop
		p.mu.Unlock()
		select {
		case <-stop:
			return
		case <-t.C:
			p.RunOnce(context.Background())
		}
	}
}

// Rounds 返回已执行轮数。
func (p *Prober) Rounds() uint64 { return p.round.Load() }
