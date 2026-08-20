// Package sampler 概率采样器：根据成功率动态调整采样率。
package sampler

import (
	"crypto/rand"
	"encoding/binary"
	"sync"
	"sync/atomic"
)

// Controller 维护一个目标速率（0..1），逐项决定保留还是丢弃。
// 速率会根据观测到的负载 / 错误信号进行自适应重新计算。
type Controller struct {
	mu        sync.RWMutex
	rate      float64
	minRate   float64
	maxRate   float64
	targetEPS int
	window    uint64 // window size for EWMA
	curEPS    atomic.Uint64
	curErr    atomic.Uint64
	decisions atomic.Uint64
	keeps     atomic.Uint64
	drops     atomic.Uint64
}

// Config 用于配置 Controller。
type Config struct {
	Rate      float64 // initial rate 0..1
	MinRate   float64
	MaxRate   float64
	TargetEPS int
	Window    uint64 // EWMA window
}

// Default 返回一份合理的默认配置。
func Default() Config {
	return Config{
		Rate: 1.0, MinRate: 0.01, MaxRate: 1.0,
		TargetEPS: 10000, Window: 1000,
	}
}

// New 构造一个 Controller。
func New(cfg Config) *Controller {
	if cfg.MinRate <= 0 {
		cfg.MinRate = 0.01
	}
	if cfg.MaxRate <= 0 {
		cfg.MaxRate = 1.0
	}
	if cfg.Window == 0 {
		cfg.Window = 1000
	}
	c := &Controller{
		minRate: cfg.MinRate,
		maxRate: cfg.MaxRate,
		targetEPS: cfg.TargetEPS,
		window: cfg.Window,
	}
	c.rate = clamp(cfg.Rate, cfg.MinRate, cfg.MaxRate)
	return c
}

// ShouldKeep 在 Controller 决定保留该条目时返回 true。
// 每次调用都会计作一次观测。
func (c *Controller) ShouldKeep() bool {
	c.mu.RLock()
	r := c.rate
	c.mu.RUnlock()
	c.decisions.Add(1)
	x := randFloat()
	if x <= r {
		c.keeps.Add(1)
		return true
	}
	c.drops.Add(1)
	return false
}

// Observe 上报一次观测。err=true 表示被保留的条目失败。
// 每累计 window 次观测后，Controller 会调整速率，
// 既要靠近 targetEPS，也会惩罚较高的错误率。
func (c *Controller) Observe(err bool) {
	c.curEPS.Add(1)
	if err {
		c.curErr.Add(1)
	}
	c.decisions.Add(1)
	w := c.window
	if w == 0 {
		w = 1000
	}
	if c.decisions.Load()%w == 0 {
		c.adjust()
	}
}

// adjust 根据当前 EPS 与错误率重新计算速率。
// EPS 相对 target 偏高 -> 降低速率。
// 错误率偏高 -> 降低速率。
// 二者均低于目标 -> 提高速率。
func (c *Controller) adjust() {
	decisions := c.decisions.Load()
	if decisions == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	eps := float64(c.curEPS.Load())
	errs := float64(c.curErr.Load())
	errRate := errs / eps
	if errRate > 0.05 {
		c.rate = c.rate * 0.8
	} else if eps > float64(c.targetEPS)*1.2 {
		c.rate = c.rate * 0.9
	} else if eps < float64(c.targetEPS)*0.8 {
		c.rate = c.rate * 1.1
	}
	c.rate = clamp(c.rate, c.minRate, c.maxRate)
}

// SetRate 手动设置速率（覆盖自适应控制，
// 直到下一次 Observe 触发 adjust 为止）。
func (c *Controller) SetRate(r float64) {
	c.mu.Lock()
	c.rate = clamp(r, c.minRate, c.maxRate)
	c.mu.Unlock()
}

// Rate 返回当前速率。
func (c *Controller) Rate() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.rate
}

// Stats 是 Controller 的快照。
type Stats struct {
	Rate      float64 `json:"rate"`
	MinRate   float64 `json:"min_rate"`
	MaxRate   float64 `json:"max_rate"`
	Decisions uint64  `json:"decisions"`
	Keeps     uint64  `json:"keeps"`
	Drops     uint64  `json:"drops"`
	KeptRatio float64 `json:"kept_ratio"`
	Errors    uint64  `json:"errors"`
	ErrRate   float64 `json:"err_rate"`
}

// Stats 返回当前快照。
func (c *Controller) Stats() Stats {
	c.mu.RLock()
	r := c.rate
	c.mu.RUnlock()
	k := c.keeps.Load()
	d := c.drops.Load()
	dec := c.decisions.Load()
	kept := 0.0
	if dec > 0 {
		kept = float64(k) / float64(dec)
	}
	eps := c.curEPS.Load()
	err := c.curErr.Load()
	errRate := 0.0
	if eps > 0 {
		errRate = float64(err) / float64(eps)
	}
	return Stats{
		Rate: r, MinRate: c.minRate, MaxRate: c.maxRate,
		Decisions: dec, Keeps: k, Drops: d,
		KeptRatio: kept, Errors: err, ErrRate: errRate,
	}
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func randFloat() float64 {
	var b [8]byte
	_, _ = rand.Read(b[:])
	n := binary.LittleEndian.Uint64(b[:])
	return float64(n>>11) / float64(1<<53)
}
