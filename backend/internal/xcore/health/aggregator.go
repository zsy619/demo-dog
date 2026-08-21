package health

// aggregator.go:Aggregator 主体与所有调度方法。
//
// Aggregator 持有 Check 表与注册顺序,提供 RunAll 批量探测与 HandleHTTP 端点。
// 使用 sync.RWMutex 保护 checks / order;时间源可注入便于测试。

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"
)

// Aggregator 持有所有检查项。
type Aggregator struct {
	mu     sync.RWMutex      // 保护 checks / order
	checks map[string]*Check // name → Check
	order  []string          // 注册顺序(用于 RunAll 时确定性顺序)
	now    func() time.Time  // 时间源(测试可注入)
}

// NewAggregator 返回一个空的聚合器。
func NewAggregator() *Aggregator {
	return &Aggregator{
		checks: make(map[string]*Check),
		now:    time.Now,
	}
}

// Register 添加一个检查项。
//
// 缺省 Threshold 设为 2 秒;缺省 Probe 永远成功。
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

// RunAll 执行所有检查并返回快照。
//
// 每个 check 单独设置 Threshold 超时;任一 critical check 失败会令整组不健康。
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

// HandleHTTP 返回一个会运行所有检查的 http.Handler。
//
// 全部健康返回 200 OK;否则返回 503 Service Unavailable。
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
