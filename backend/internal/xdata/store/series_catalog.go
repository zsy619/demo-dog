package store

import (
	"sort"
	"sync"
	"time"
)

// SeriesEntry 是某指标观察到的一个唯一标签集。
type SeriesEntry struct {
	Service string            `json:"service"`
	Name    string            `json:"name"`
	Labels  map[string]string `json:"labels,omitempty"`
	LastMs  int64             `json:"last_ms"`
}

// MetricCard 汇总一个指标名称。
type MetricCard struct {
	Name        string `json:"name"`
	Series      int    `json:"series"`
	Samples     int    `json:"samples"`
	Services    int    `json:"services"`
	FirstSeenMs int64  `json:"first_seen_ms,omitempty"`
	LastSeenMs  int64  `json:"last_seen_ms,omitempty"`
}

// SeriesCatalog 遍历内存热/冷指标缓冲，并
// produces a catalog of (metric, series).
type SeriesCatalog struct {
	mu   sync.RWMutex
	d    *Doris
	ttl  time.Duration
	last time.Time
	memo []MetricCard
}

func (d *Doris) NewSeriesCatalog(ttl time.Duration) *SeriesCatalog {
	if ttl <= 0 {
		ttl = 5 * time.Second
	}
	return &SeriesCatalog{d: d, ttl: ttl}
}

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

func (d *Doris) metricKeyOrder(name string) []string {
	out := make([]string, 0, len(d.hotMetrics))
	for k := range d.hotMetrics {
		if _, n := splitMetricKey(k); n == name {
			out = append(out, k)
		}
	}
	return out
}

func splitMetricKey(k string) (svc, name string) {
	for i := 0; i < len(k); i++ {
		if k[i] == '|' {
			return k[:i], k[i+1:]
		}
	}
	return k, ""
}

func labelsKey(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]byte, 0, 32)
	for _, k := range keys {
		out = append(out, k...)
		out = append(out, '=')
		out = append(out, m[k]...)
		out = append(out, ';')
	}
	return string(out)
}

func copyLabels(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
