package store

// series_catalog.go:SeriesCatalog 主题。
//
// 遍历 Doris 的内存 hot/cold metrics 缓冲,
// 产出 (metric, 标签集) 清单;带 ttl memo。

import (
	"sort"
	"sync"
	"time"
)

// SeriesCatalog 遍历内存热/冷指标缓冲,
// 并产出 (metric, 序列) 清单。
//
// 带 ttl 缓存:ttl 内直接返回缓存结果。
type SeriesCatalog struct {
	mu   sync.RWMutex // 保护 memo / last
	d    *Doris       // 源数据
	ttl  time.Duration // 缓存有效期
	last time.Time     // 上次重算时间
	memo []MetricCard  // 缓存结果
}

// NewSeriesCatalog 构造一个 catalog,ttl <= 0 默认为 5 秒。
func (d *Doris) NewSeriesCatalog(ttl time.Duration) *SeriesCatalog {
	if ttl <= 0 {
		ttl = 5 * time.Second
	}
	return &SeriesCatalog{d: d, ttl: ttl}
}

// Series 返回所有指标的汇总(缓存命中直接返回)。
func (c *SeriesCatalog) Series() []MetricCard {
	c.mu.RLock()
	if time.Since(c.last) < c.ttl && c.memo != nil {
		out := c.memo
		c.mu.RUnlock()
		return out
	}
	c.mu.RUnlock()
	return c.recompute()
}

// ForMetric 返回指定指标名下的所有唯一标签集。
//
// 按 service 升序、LastMs 降序排序;limit > 0 截断。
func (c *SeriesCatalog) ForMetric(name string, limit int) []SeriesEntry {
	d := c.d
	d.muMetrics.RLock()
	defer d.muMetrics.RUnlock()
	seen := map[string]map[string]string{}
	last := map[string]int64{}
	for _, key := range d.metricKeyOrder(name) {
		for _, p := range d.hotMetrics[key] {
			if p.Name != name {
				continue
			}
			lblKey := labelsKey(p.Labels)
			if _, ok := seen[lblKey]; !ok {
				seen[lblKey] = copyLabels(p.Labels)
			}
			if p.Timestamp.UnixMilli() > last[lblKey] {
				last[lblKey] = p.Timestamp.UnixMilli()
			}
		}
	}
	out := make([]SeriesEntry, 0, len(seen))
	for lk, lbls := range seen {
		service := ""
		if v, ok := lbls["service"]; ok {
			service = v
		}
		if service == "" {
			service = lbls["service.name"]
		}
		out = append(out, SeriesEntry{
			Service: service,
			Name:    name,
			Labels:  lbls,
			LastMs:  last[lk],
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Service != out[j].Service {
			return out[i].Service < out[j].Service
		}
		return out[i].LastMs > out[j].LastMs
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// recompute 重新遍历 hot metrics 缓冲并构造 MetricCard 列表。
//
// 结果按 series 数降序、名称升序排序。
func (c *SeriesCatalog) recompute() []MetricCard {
	d := c.d
	d.muMetrics.RLock()
	defer d.muMetrics.RUnlock()

	type agg struct {
		series   map[string]struct{}
		services map[string]struct{}
		samples  int
		firstMs  int64
		lastMs   int64
	}
	byName := map[string]*agg{}
	for key, pts := range d.hotMetrics {
		svc, name := splitMetricKey(key)
		a := byName[name]
		if a == nil {
			a = &agg{
				series:   map[string]struct{}{},
				services: map[string]struct{}{},
			}
			byName[name] = a
		}
		a.services[svc] = struct{}{}
		for _, p := range pts {
			a.series[labelsKey(p.Labels)] = struct{}{}
			a.samples++
			ms := p.Timestamp.UnixMilli()
			if a.firstMs == 0 || ms < a.firstMs {
				a.firstMs = ms
			}
			if ms > a.lastMs {
				a.lastMs = ms
			}
		}
	}
	out := make([]MetricCard, 0, len(byName))
	for name, a := range byName {
		out = append(out, MetricCard{
			Name:        name,
			Series:      len(a.series),
			Samples:     a.samples,
			Services:    len(a.services),
			FirstSeenMs: a.firstMs,
			LastSeenMs:  a.lastMs,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Series != out[j].Series {
			return out[i].Series > out[j].Series
		}
		return out[i].Name < out[j].Name
	})
	c.mu.Lock()
	c.memo = out
	c.last = time.Now()
	c.mu.Unlock()
	return out
}
