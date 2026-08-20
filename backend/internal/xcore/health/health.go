// Package health 健康检查：探测外部依赖并汇总健康状态。
package health

// 健康检查聚合器。
//
// Round 60 把所有健康检查汇聚到统一的 Status 表层，供 liveness 与 readiness 探针消费。
// 支持两种检查：
//
//   - Ping 检查：同步的 HTTP / TCP / DB ping。
//   - Worker 检查：上报协程池的 in-flight / 队列深度，并为每个检查配置阈值。
//
// 仅当所有检查都为 "ok" 时，整体 Status 才为 "ok"。
// Snapshot 采用 JSON 稳定结构，因此 /healthz / /readyz / /debug/health 等端点
// 都能输出相同形态的数据。

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

// Check 表示一个具名健康探测。
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

// Aggregator 持有所有检查项。
type Aggregator struct {
	mu     sync.RWMutex
	checks map[string]*Check
	order  []string
	now    func() time.Time
}

// NewAggregator 返回一个空的聚合器。
func NewAggregator() *Aggregator {
	return &Aggregator{
		checks: make(map[string]*Check),
		now:    time.Now,
	}
}

// Register 添加一个检查项。
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

// Remove 删除一个检查项。
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

// RunAll 执行所有检查并返回快照。
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

// Snapshot 是 JSON 稳定的结果。
type Snapshot struct {
	At       time.Time          `json:"at"`
	Healthy  bool               `json:"healthy"`
	Critical bool               `json:"critical"`
	OK       int                `json:"ok"`
	Failed   int                `json:"failed"`
	Items    map[string]*Check  `json:"items"`
}

// Healthy 报告是否所有检查都正常。
func (s Snapshot) Healthy_() bool { return s.Healthy }

// HandleHTTP 返回一个会运行所有检查的 http.Handler。
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

// HTTPCheck 构造一个会访问指定 URL 的 Check。
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

// TCPCheck 构造一个会建立 TCP 连接的 Check。
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

// WorkerCheck 上报具名 Worker 池的健康状态。
// 当 active <= max 时 probe 返回 nil。
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
