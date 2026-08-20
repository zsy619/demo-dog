// Package timeoutlearner 自适应超时：根据历史数据动态调整超时阈值。
package timeoutlearner

import (
	"math"
	"sync"
	"time"
)

// Learner tracks per-host response times and recommends a
// timeout. The current implementation is an EWMA over the
// p95-ish tail plus a safety factor.
type Learner struct {
	mu       sync.Mutex
	hosts    map[string]*hostStat
	alpha    float64
	safety   float64
	min      time.Duration
	max      time.Duration
	fallback time.Duration
}

type hostStat struct {
	ewmaMs float64
	count  int
	last   time.Time
}

// Config is the Learner constructor options.
type Config struct {
	EWMAAlpha  float64       // smoothing 0..1; default 0.2
	Safety     float64       // multiplier on EWMA; default 2.0
	Min        time.Duration // floor; default 100ms
	Max        time.Duration // ceiling; default 30s
	Fallback   time.Duration // before any sample; default 1s
}

// New creates a Learner.
func New(c Config) *Learner {
	if c.EWMAAlpha <= 0 {
		c.EWMAAlpha = 0.2
	}
	if c.Safety <= 0 {
		c.Safety = 2.0
	}
	if c.Min <= 0 {
		c.Min = 100 * time.Millisecond
	}
	if c.Max <= 0 {
		c.Max = 30 * time.Second
	}
	if c.Fallback <= 0 {
		c.Fallback = time.Second
	}
	return &Learner{
		hosts:    make(map[string]*hostStat),
		alpha:    c.EWMAAlpha,
		safety:   c.Safety,
		min:      c.Min,
		max:      c.Max,
		fallback: c.Fallback,
	}
}

// Observe records one successful sample.
func (l *Learner) Observe(host string, dur time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	st, ok := l.hosts[host]
	if !ok {
		st = &hostStat{}
		l.hosts[host] = st
	}
	ms := float64(dur.Microseconds()) / 1000.0
	if st.count == 0 {
		st.ewmaMs = ms
	} else {
		st.ewmaMs = l.alpha*ms + (1-l.alpha)*st.ewmaMs
	}
	st.count++
	st.last = time.Now()
}

// Timeout returns the recommended timeout for host.
func (l *Learner) Timeout(host string) time.Duration {
	l.mu.Lock()
	st, ok := l.hosts[host]
	l.mu.Unlock()
	if !ok || st.count == 0 {
		return l.fallback
	}
	ms := st.ewmaMs * l.safety
	d := time.Duration(ms * float64(time.Millisecond))
	if d < l.min {
		return l.min
	}
	if d > l.max {
		return l.max
	}
	return d
}

// Forget removes host's samples.
func (l *Learner) Forget(host string) {
	l.mu.Lock()
	delete(l.hosts, host)
	l.mu.Unlock()
}

// Stats is one host's view.
type Stats struct {
	EWMA  float64 `json:"ewma_ms"`
	Count int     `json:"count"`
}

// Stats returns a copy of host's stats.
func (l *Learner) Stats(host string) Stats {
	l.mu.Lock()
	defer l.mu.Unlock()
	st, ok := l.hosts[host]
	if !ok {
		return Stats{}
	}
	return Stats{EWMA: st.ewmaMs, Count: st.count}
}

// Snapshot returns all host stats.
func (l *Learner) Snapshot() map[string]Stats {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make(map[string]Stats, len(l.hosts))
	for k, st := range l.hosts {
		out[k] = Stats{EWMA: st.ewmaMs, Count: st.count}
	}
	return out
}

// Reset clears all learned data.
func (l *Learner) Reset() {
	l.mu.Lock()
	l.hosts = make(map[string]*hostStat)
	l.mu.Unlock()
}

// Sanity is a self-test that the EWMA math is bounded.
func Sanity() bool {
	l := New(Config{EWMAAlpha: 0.2, Safety: 2.0, Min: 100 * time.Millisecond, Max: 30 * time.Second, Fallback: time.Second})
	for i := 0; i < 100; i++ {
		l.Observe("a", 200*time.Millisecond)
	}
	return math.Abs(l.Stats("a").EWMA-200) < 1
}
