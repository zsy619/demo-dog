// Package health 健康检查：探测外部依赖并汇总健康状态。
package health

// Health aggregator.
//
// Round 60 wires every health check into a single Status
// surface that the liveness + readiness probes consume.
// Supports two kinds of checks:
//
//   - Ping checks: synchronous HTTP / TCP / DB pings.
//   - Worker checks: report the in-flight / queue depth of
//     a goroutine pool, with thresholds per check.
//
// The overall Status is "ok" iff every check is "ok". The
// Snapshot is JSON-stable so /healthz / /readyz / /debug/health
// can all dump the same shape.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

// Check is one named health probe.
type Check struct {
	Name      string
	Critical  bool
	Probe     func(ctx context.Context) error
	Threshold time.Duration
	Status    string
	Error     string
	Took      time.Duration
	At        time.Time
}

// Aggregator owns the check table.
type Aggregator struct {
	mu     sync.RWMutex
	checks map[string]*Check
	order  []string
	now    func() time.Time
}

// NewAggregator returns an empty aggregator.
func NewAggregator() *Aggregator {
	return &Aggregator{
		checks: make(map[string]*Check),
		now:    time.Now,
	}
}

// Register adds a check.
func (a *Aggregator) Register(c *Check) {
	if c.Threshold == 0 {
		c.Threshold = 2 * time.Second
	}
	if c.Probe == nil {
		c.Probe = func(ctx context.Context) error { return nil }
	}
	a.mu.Lock()
	if _, ok := a.checks[c.Name]; !ok {
		a.order = append(a.order, c.Name)
	}
	a.checks[c.Name] = c
	a.mu.Unlock()
}

// Remove drops a check.
func (a *Aggregator) Remove(name string) {
	a.mu.Lock()
	if _, ok := a.checks[name]; ok {
		delete(a.checks, name)
		a.order = removeString(a.order, name)
	}
	a.mu.Unlock()
}

func removeString(s []string, t string) []string {
	out := s[:0]
	for _, x := range s {
		if x != t {
			out = append(out, x)
		}
	}
	return out
}

// RunAll executes every check and returns the snapshot.
func (a *Aggregator) RunAll(parent context.Context) Snapshot {
	a.mu.RLock()
	checks := make([]*Check, len(a.order))
	for i, n := range a.order {
		checks[i] = a.checks[n]
	}
	a.mu.RUnlock()
	res := Snapshot{At: a.now(), Items: make(map[string]*Check, len(checks))}
	for _, c := range checks {
		ctx, cancel := context.WithTimeout(parent, c.Threshold)
		start := a.now()
		err := c.Probe(ctx)
		took := a.now().Sub(start)
		cancel()
		c.Status = "ok"
		c.Error = ""
		c.Took = took
		c.At = a.now()
		if err != nil {
			c.Status = "failed"
			c.Error = err.Error()
			res.Failed++
		} else {
			res.OK++
		}
		res.Items[c.Name] = c
	}
	for _, c := range checks {
		if c.Critical && c.Status != "ok" {
			res.Critical = false
			break
		}
	}
	if res.Failed == 0 {
		res.Healthy = true
	}
	return res
}

// Snapshot is the JSON-stable result.
type Snapshot struct {
	At       time.Time          `json:"at"`
	Healthy  bool               `json:"healthy"`
	Critical bool               `json:"critical"`
	OK       int                `json:"ok"`
	Failed   int                `json:"failed"`
	Items    map[string]*Check  `json:"items"`
}

// Healthy reports whether every check is ok.
func (s Snapshot) Healthy_() bool { return s.Healthy }

// HandleHTTP returns an http.Handler that runs all checks.
func (a *Aggregator) HandleHTTP() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		snap := a.RunAll(r.Context())
		w.Header().Set("Content-Type", "application/json")
		if !snap.Healthy {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		b, _ := json.MarshalIndent(snap, "", "  ")
		io.WriteString(w, string(b))
	})
}

// HTTPCheck builds a Check that hits a URL.
func HTTPCheck(name, url string, critical bool) *Check {
	return &Check{
		Name: name, Critical: critical,
		Probe: func(ctx context.Context) error {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				return err
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode >= 500 {
				return fmt.Errorf("status %d", resp.StatusCode)
			}
			return nil
		},
	}
}

// TCPCheck builds a Check that opens a TCP connection.
func TCPCheck(name, addr string, critical bool) *Check {
	return &Check{
		Name: name, Critical: critical,
		Probe: func(ctx context.Context) error {
			d := net.Dialer{Timeout: 2 * time.Second}
			conn, err := d.DialContext(ctx, "tcp", addr)
			if err != nil {
				return err
			}
			conn.Close()
			return nil
		},
	}
}

// WorkerCheck reports the health of a named worker pool.
// The probe returns nil if active <= max.
func WorkerCheck(name string, active, max int, critical bool) *Check {
	return &Check{
		Name: name, Critical: critical,
		Probe: func(ctx context.Context) error {
			if active > max {
				return fmt.Errorf("active %d > max %d", active, max)
			}
			return nil
		},
	}
}
