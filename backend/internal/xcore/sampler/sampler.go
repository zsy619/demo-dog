// Package sampler 概率采样器：根据成功率动态调整采样率。
package sampler

import (
	"crypto/rand"
	"encoding/binary"
	"sync"
	"sync/atomic"
)

// Controller keeps a target rate (0..1) and decides per-item
// whether to keep or drop. The rate is recomputed
// adaptively from observed load / error signals.
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

// Config configures the controller.
type Config struct {
	Rate      float64 // initial rate 0..1
	MinRate   float64
	MaxRate   float64
	TargetEPS int
	Window    uint64 // EWMA window
}

// Default returns a sane default config.
func Default() Config {
	return Config{
		Rate: 1.0, MinRate: 0.01, MaxRate: 1.0,
		TargetEPS: 10000, Window: 1000,
	}
}

// New constructs a Controller.
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

// ShouldKeep returns true when the controller decides to
// keep the item. Each call also counts as one observation.
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

// Observe signals an observation. err=true means the kept
// item failed. After every `window` observations the
// controller adjusts the rate to stay near targetEPS while
// penalising high error ratios.
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

// adjust recomputes the rate from current EPS and error rate.
// High EPS relative to target -> drop rate. High error rate
// -> drop rate. Both < targets -> raise rate.
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

// SetRate sets the rate manually (overrides adaptive control
// until next Observe triggers adjust).
func (c *Controller) SetRate(r float64) {
	c.mu.Lock()
	c.rate = clamp(r, c.minRate, c.maxRate)
	c.mu.Unlock()
}

// Rate returns the current rate.
func (c *Controller) Rate() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.rate
}

// Stats is the controller snapshot.
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

// Stats returns the snapshot.
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
