// Package xbilling 提供按租户、按指标、按周期的用量计量与计费导出。
//
// 多租户 SaaS(W1.5)的核心:每个 (tenant, metric) 都有一组
// 周期桶(默认 monthly = "YYYY-MM"),Record() 累加,
// Query() 返回 per-period totals。
//
// 持久化(counter 快照)沿用 xpersistence.KV 写穿模式,
// 与 tenants/admin-keys/OIDC/breaker/retention/quota/webhooks
// 共享同一份 control.json。键命名:
//
//	metering/{period}/{tenant}/{metric} → JSON PeriodTotal
//
// 启动时 load() 把所有 period 读回内存,客户端仍
// 可以纯内存模式(NewCounter)运行测试或单元测试。
package xbilling

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xpersistence"
)

// Metric 命名一个可计费的资源(invocations, bytes_in, span_qps ...)。
type Metric string

// PeriodFormat 是周期桶的时间格式。默认按月分桶。
const PeriodFormat = "2006-01"

// PeriodOf 返回 t 所在的 YYYY-MM 字符串。
func PeriodOf(t time.Time) string { return t.UTC().Format(PeriodFormat) }

// PeriodTotal 是单 (period, tenant, metric) 的累计值。
type PeriodTotal struct {
	Tenant     string    `json:"tenant"`
	Metric     string    `json:"metric"`
	Period     string    `json:"period"` // YYYY-MM
	Value      int64     `json:"value"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Usage 是查询结果的视图,按 (tenant, metric) 列出所有 period。
type Usage struct {
	Tenant  string           `json:"tenant"`
	Metric  string           `json:"metric"`
	Periods map[string]int64 `json:"periods"` // period → value
	Total   int64            `json:"total"`
}

// Meter 暴露用量计数的最小接口。
//
// 实现:Counter(纯内存)、Aggregator(带 KV 持久化)。
type Meter interface {
	Record(tenant string, metric Metric, delta int64, at time.Time)
	Query(tenant, metric, period string) (int64, bool)
	UsageFor(tenant string) []Usage
	All() []PeriodTotal
}

// Counter 是纯内存版 Meter,适合单测或
// 不需要跨进程恢复的场景。
type Counter struct {
	mu     sync.RWMutex
	totals map[string]*PeriodTotal // key = tenant + "|" + metric + "|" + period
	now    func() time.Time
}

// NewCounter 返回一个空 Counter。
func NewCounter() *Counter {
	return &Counter{
		totals: make(map[string]*PeriodTotal),
		now:    time.Now,
	}
}

func (c *Counter) nowTime() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

// key 是内部 map 用的紧凑字符串。
func key(tenant string, metric Metric, period string) string {
	return tenant + "|" + string(metric) + "|" + period
}

// Record 累加一个 delta。
func (c *Counter) Record(tenant string, metric Metric, delta int64, at time.Time) {
	if tenant == "" || metric == "" {
		return
	}
	if at.IsZero() {
		at = c.nowTime()
	}
	period := PeriodOf(at)
	c.mu.Lock()
	defer c.mu.Unlock()
	k := key(tenant, metric, period)
	t, ok := c.totals[k]
	if !ok {
		t = &PeriodTotal{
			Tenant: tenant,
			Metric: string(metric),
			Period: period,
		}
		c.totals[k] = t
	}
	t.Value += delta
	t.UpdatedAt = c.nowTime()
}

// Query 返回单 (tenant, metric, period) 的当前计数。
func (c *Counter) Query(tenant, metric, period string) (int64, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	t, ok := c.totals[key(tenant, Metric(metric), period)]
	if !ok {
		return 0, false
	}
	return t.Value, true
}

// UsageFor 列出该 tenant 的所有 (metric → period → value) 视图。
func (c *Counter) UsageFor(tenant string) []Usage {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// 按 (metric, period) 聚合。
	type uk struct{ metric, period string }
	acc := make(map[uk]int64)
	for _, t := range c.totals {
		if t.Tenant != tenant {
			continue
		}
		acc[uk{t.Metric, t.Period}] += t.Value
	}
	byMetric := make(map[string]*Usage)
	for k, val := range acc {
		u, ok := byMetric[k.metric]
		if !ok {
			u = &Usage{
				Tenant:  tenant,
				Metric:  k.metric,
				Periods: make(map[string]int64),
			}
			byMetric[k.metric] = u
		}
		u.Periods[k.period] += val
		u.Total += val
	}
	out := make([]Usage, 0, len(byMetric))
	for _, u := range byMetric {
		out = append(out, *u)
	}
	return out
}

// All 返回所有 PeriodTotal 列表。
func (c *Counter) All() []PeriodTotal {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]PeriodTotal, 0, len(c.totals))
	for _, t := range c.totals {
		out = append(out, *t)
	}
	return out
}

// Aggregator 把 Counter 接到 xpersistence.KV 上:
// 启动时 load(),Record() 写穿。
type Aggregator struct {
	mu    sync.RWMutex
	inner *Counter
	kv    xpersistence.KV
}

// NewAggregator 返回一个带持久化的聚合器。
func NewAggregator(ctx context.Context, kv xpersistence.KV) (*Aggregator, error) {
	if kv == nil {
		return nil, errors.New("xbilling: kv is nil")
	}
	a := &Aggregator{
		inner: NewCounter(),
		kv:    kv,
	}
	if err := a.load(ctx); err != nil {
		return nil, fmt.Errorf("xbilling: load: %w", err)
	}
	return a, nil
}

// SetKV 用于测试或后注入 KV。
func (a *Aggregator) SetKV(ctx context.Context, kv xpersistence.KV) error {
	if kv == nil {
		return errors.New("xbilling: kv is nil")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.inner.totals) > 0 {
		return errors.New("xbilling: SetKV on non-empty aggregator")
	}
	a.kv = kv
	return a.load(ctx)
}

// load 从 KV 读所有 PeriodTotal 并放入 inner counter。
func (a *Aggregator) load(ctx context.Context) error {
	keys, err := a.kv.List(ctx, "metering/")
	if err != nil {
		return err
	}
	for _, k := range keys {
		raw, err := a.kv.Get(ctx, k)
		if err != nil {
			continue
		}
		var t PeriodTotal
		if err := json.Unmarshal(raw, &t); err != nil {
			continue
		}
		if t.Tenant == "" || t.Metric == "" || t.Period == "" {
			continue
		}
		a.inner.totals[key(t.Tenant, Metric(t.Metric), t.Period)] = &t
	}
	return nil
}

// persist 写一条 PeriodTotal 到 KV。
func (a *Aggregator) persist(t PeriodTotal) error {
	if a.kv == nil {
		return nil
	}
	raw, err := json.Marshal(t)
	if err != nil {
		return err
	}
	return a.kv.Set(context.Background(), kvKey(t), raw)
}

func kvKey(t PeriodTotal) string {
	return "metering/" + t.Period + "/" + t.Tenant + "/" + t.Metric
}

// Record 累加并写穿到 KV。
func (a *Aggregator) Record(tenant string, metric Metric, delta int64, at time.Time) {
	if tenant == "" || metric == "" {
		return
	}
	if at.IsZero() {
		at = a.inner.nowTime()
	}
	period := PeriodOf(at)
	a.mu.Lock()
	defer a.mu.Unlock()
	k := key(tenant, metric, period)
	t, ok := a.inner.totals[k]
	if !ok {
		t = &PeriodTotal{
			Tenant: tenant,
			Metric: string(metric),
			Period: period,
		}
		a.inner.totals[k] = t
	}
	t.Value += delta
	t.UpdatedAt = a.inner.nowTime()
	_ = a.persist(*t)
}

// Query 代理到 inner。
func (a *Aggregator) Query(tenant, metric, period string) (int64, bool) {
	return a.inner.Query(tenant, metric, period)
}

// UsageFor 代理到 inner。
func (a *Aggregator) UsageFor(tenant string) []Usage {
	return a.inner.UsageFor(tenant)
}

// All 代理到 inner。
func (a *Aggregator) All() []PeriodTotal {
	return a.inner.All()
}

// Counter 暴露底层 Counter,用于内部访问或在测试中复用。
func (a *Aggregator) Counter() *Counter { return a.inner }

// EncodeCSV 把 usage 列表编码成 CSV 字节流。
//
// 格式:period,tenant,metric,value,updated_at
// 每行一条,适合导入账单/Excel。
func EncodeCSV(rows []PeriodTotal) []byte {
	var b strings.Builder
	b.WriteString("period,tenant,metric,value,updated_at\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "%s,%s,%s,%d,%s\n",
			r.Period, r.Tenant, r.Metric, r.Value,
			r.UpdatedAt.UTC().Format(time.RFC3339Nano))
	}
	return []byte(b.String())
}
