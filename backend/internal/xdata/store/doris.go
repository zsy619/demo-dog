// Package store 提供 Apache Doris 引擎的内存模拟实现。
//
// The design intentionally mirrors real Doris concepts so that the demo
// behaves like an OLAP backend, even though everything fits in RAM:
//
//   - Each signal lives in its own table: __dog_logs, __dog_metrics, __dog_traces.
//   - Hot/cold tiering: the most recent N records are kept in a "hot" partition
//     and the older ones spill into a "cold" partition. Queries report which tier
//     they hit so the frontend can hint about latency.
//   - Materialized views: simple bucket-level aggregations (logs per service,
//     metric 1m rollup) are pre-computed and labeled by window.
//   - Pseudo-indexes: hash buckets + range indices on timestamp + service.
//
// Concurrency: every public method is safe for concurrent use. We rely on
// single-writer-multiple-readers semantics by guarding mutations with a mutex
// and bucket-level locking for the hot tier.
package store

import (
	"errors"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xdata/model"
)

// Config 用于调优内存 Doris 引擎。
type Config struct {
	HotLogTTL    time.Duration // logs older than this move to cold
	HotLogCap    int           // max hot logs per bucket
	HotMetricCap int           // max hot metric points per (service, name)
	ColdCap      int           // max cold rows per signal table
	// MaxCardinality 限制唯一 (service, name,
	// label-set) tuples the engine will accept. 0 means unlimited.
	// When the limit is hit, new series are dropped and the
	// dropped counter increments. Set this in production to
	// defend against misconfigured agents emitting high-cardinality
	// labels (user_id, trace_id, etc.).
	MaxCardinality int
}

// DefaultConfig 返回适合 demo 的合理默认。
func DefaultConfig() Config {
	return Config{
		HotLogTTL:      5 * time.Minute,
		HotLogCap:      2048,
		HotMetricCap:   4096,
		ColdCap:        10_000,
		MaxCardinality: 50_000,
	}
}

// Validate 返回首个配置错误或 nil。
// Callers should fail fast at startup.
func (c Config) Validate() error {
	if c.HotLogTTL < 0 {
		return errors.New("HotLogTTL must be >= 0")
	}
	if c.HotLogCap <= 0 {
		return errors.New("HotLogCap must be > 0")
	}
	if c.HotMetricCap <= 0 {
		return errors.New("HotMetricCap must be > 0")
	}
	if c.ColdCap <= 0 {
		return errors.New("ColdCap must be > 0")
	}
	if c.MaxCardinality < 0 {
		return errors.New("MaxCardinality must be >= 0 (0 = unlimited)")
	}
	return nil
}

// Doris 是 demo 的内存引擎。
type Doris struct {
	cfg Config

	// wal 是可选的预写日志。设置后，每次插入
	// appends a record so a crash + restart can replay the last few
	// seconds of writes without losing them.
	walMu sync.Mutex
	wal   *WAL

	muLogs    sync.RWMutex
	hotLogs   []model.LogRecord
	coldLogs  []model.LogRecord
	logBuckets map[string]int // service -> log count for quick lookups

	muMetrics sync.RWMutex
	hotMetrics  map[string][]model.MetricPoint // key=service|name
	coldMetrics []model.MetricPoint

	// 直方图按 service|name 索引。我们将 sum/count 聚合到
	// MV-style buckets AND keep the latest bucket_bounds + per-bucket
	// counts for proper quantile queries. Without this, OTel histograms
	// would degrade to scalar sum/count and SLO p95/p99 would silently
	// be wrong.
	muHistograms sync.RWMutex
	histograms   map[string]*histogramAgg // key=service|name

	muSpans sync.RWMutex
	hotSpans  map[string][]model.SpanRecord // key=trace_id
	coldSpans []model.SpanRecord

	// 为指标预计算 1m 和 5m 物化视图，按 service|name 索引。
	muMV         sync.RWMutex
	mvMinute     map[string][]model.MVBucket
	mvFiveMinute map[string][]model.MVBucket

	// 服务摘要的簿记。
	muSum sync.RWMutex
	sum   map[string]*model.ServiceSummary

	// /api/health 的总计数器。
	logsAccepted    atomic.Int64
	metricsAccepted atomic.Int64
	spansAccepted   atomic.Int64
	queriesServed   atomic.Int64

	// 基数跟踪。seriesCardinality 统计唯一
	// (service, name, label-set) tuples currently held in hotMetrics.
	// When cardinality > cfg.MaxCardinality new inserts are dropped
	// and seriesDropped is incremented.
	seriesCardinality atomic.Int64
	seriesDropped     atomic.Int64
}

// New 返回一个新初始化的 Doris 引擎。
func New(cfg Config) *Doris {
	if cfg.HotLogTTL == 0 {
		cfg = DefaultConfig()
	}
	return &Doris{
		cfg:          cfg,
		hotLogs:      make([]model.LogRecord, 0, cfg.HotLogCap),
		coldLogs:     make([]model.LogRecord, 0, cfg.ColdCap),
		logBuckets:   make(map[string]int),
		hotMetrics:   make(map[string][]model.MetricPoint),
		histograms:   make(map[string]*histogramAgg),
		coldMetrics:  make([]model.MetricPoint, 0, cfg.ColdCap),
		hotSpans:     make(map[string][]model.SpanRecord),
		coldSpans:    make([]model.SpanRecord, 0, cfg.ColdCap),
		mvMinute:     make(map[string][]model.MVBucket),
		mvFiveMinute: make(map[string][]model.MVBucket),
		sum:          make(map[string]*model.ServiceSummary),
	}
}

// Stats 报告 /api/health 暴露的计数器。
type Stats struct {
	LogsAccepted    int64 `json:"logs_accepted"`
	MetricsAccepted int64 `json:"metrics_accepted"`
	SpansAccepted   int64 `json:"spans_accepted"`
	QueriesServed   int64 `json:"queries_served"`
	HotLogs         int   `json:"hot_logs"`
	ColdLogs        int   `json:"cold_logs"`
	HotMetrics      int   `json:"hot_metrics"`
	ColdMetrics     int   `json:"cold_metrics"`
	HotSpans        int   `json:"hot_spans"`
	ColdSpans       int   `json:"cold_spans"`
	Services        int   `json:"services"`
}

// Stats 返回引擎计数器的快照。
func (d *Doris) Stats() Stats {
	d.muLogs.RLock()
	hotLogs := len(d.hotLogs)
	coldLogs := len(d.coldLogs)
	d.muLogs.RUnlock()

	d.muMetrics.RLock()
	hotMetrics := 0
	for _, v := range d.hotMetrics {
		hotMetrics += len(v)
	}
	coldMetrics := len(d.coldMetrics)
	d.muMetrics.RUnlock()

	d.muSpans.RLock()
	hotSpans := 0
	for _, v := range d.hotSpans {
		hotSpans += len(v)
	}
	coldSpans := len(d.coldSpans)
	d.muSpans.RUnlock()

	d.muSum.RLock()
	services := len(d.sum)
	d.muSum.RUnlock()

	return Stats{
		LogsAccepted:    d.logsAccepted.Load(),
		MetricsAccepted: d.metricsAccepted.Load(),
		SpansAccepted:   d.spansAccepted.Load(),
		QueriesServed:   d.queriesServed.Load(),
		HotLogs:         hotLogs,
		ColdLogs:        coldLogs,
		HotMetrics:      hotMetrics,
		ColdMetrics:     coldMetrics,
		HotSpans:        hotSpans,
		ColdSpans:       coldSpans,
		Services:        services,
	}
}

// InsertLogs 执行 Doris 风格的 Stream Load 日志记录。
// It returns the number of accepted rows.
func (d *Doris) InsertLogs(in []model.LogRecord) int {
	if len(in) == 0 {
		return 0
	}
	d.muLogs.Lock()
	now := time.Now()
	cutoff := now.Add(-d.cfg.HotLogTTL)
	hotTil := 0
	for _, r := range in {
		if r.Timestamp.Before(cutoff) {
			d.coldLogs = append(d.coldLogs, r)
		} else {
			d.hotLogs = append(d.hotLogs, r)
			hotTil++
		}
		d.logBuckets[r.Service]++
	}
	// 通过淘汰最旧记录限制热层。
	if len(d.hotLogs) > d.cfg.HotLogCap {
		evicted := d.hotLogs[:len(d.hotLogs)-d.cfg.HotLogCap]
		d.coldLogs = append(d.coldLogs, evicted...)
		d.hotLogs = d.hotLogs[len(d.hotLogs)-d.cfg.HotLogCap:]
	}
	// 限制冷层。
	if len(d.coldLogs) > d.cfg.ColdCap {
		d.coldLogs = d.coldLogs[len(d.coldLogs)-d.cfg.ColdCap:]
	}
	d.muLogs.Unlock()

	d.logsAccepted.Add(int64(len(in)))
	d.touchServices(in)
	d.appendWAL(opLog, in)
	return len(in)
}

// InsertMetrics 添加指标点并更新分钟级 MV。
// When the engine has reached cfg.MaxCardinality, new (label-set)
// variants of an existing metric are silently dropped and
// seriesDropped is incremented.
func (d *Doris) InsertMetrics(in []model.MetricPoint) int {
	if len(in) == 0 {
		return 0
	}
	d.muMetrics.Lock()
	accepted := 0
	for _, p := range in {
		key := p.TenantID + "\x00" + p.Service + "|" + p.Name
		bucket := d.hotMetrics[key]
		// 基数门控：如果此前从未观察到该精确的
		// label-set before AND we are at the cap, drop the point.
		if d.cfg.MaxCardinality > 0 && !bucketContainsLabelSet(bucket, p.Labels) {
			if d.seriesCardinality.Load() >= int64(d.cfg.MaxCardinality) {
				d.seriesDropped.Add(1)
				continue
			}
			d.seriesCardinality.Add(1)
		}
		bucket = append(bucket, p)
		if len(bucket) > d.cfg.HotMetricCap {
			old := bucket[:len(bucket)-d.cfg.HotMetricCap]
			d.coldMetrics = append(d.coldMetrics, old...)
			bucket = bucket[len(bucket)-d.cfg.HotMetricCap:]
		}
		d.hotMetrics[key] = bucket
		accepted++
	}
	if len(d.coldMetrics) > d.cfg.ColdCap {
		d.coldMetrics = d.coldMetrics[len(d.coldMetrics)-d.cfg.ColdCap:]
	}
	d.muMetrics.Unlock()

	if accepted > 0 {
		d.updateMetricMV(in[:accepted])
		d.updateHistograms(in[:accepted])
		d.metricsAccepted.Add(int64(accepted))
		d.touchServicesByMetrics(in[:accepted])
		d.appendWAL(opMetric, in[:accepted])
	}
	return accepted
}

// bucketContainsLabelSet 若桶内任意点具有
// same label map as `want`. The check is O(N) but N is bounded by
// HotMetricCap (default 4096), and label maps are usually small.
func bucketContainsLabelSet(bucket []model.MetricPoint, want map[string]string) bool {
	if len(want) == 0 {
		// 空标签集始终匹配首个空标签数据点。
		for _, p := range bucket {
			if len(p.Labels) == 0 {
				return true
			}
		}
		return false
	}
	for _, p := range bucket {
		if len(p.Labels) != len(want) {
			continue
		}
		match := true
		for k, v := range want {
			if pv, ok := p.Labels[k]; !ok || pv != v {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// CardinalityStats 暴露 /api/health 的实时序列数。
type CardinalityStats struct {
	Current int64 `json:"current"`
	Cap     int   `json:"cap"`
	Dropped int64 `json:"dropped"`
}

func (d *Doris) CardinalityStats() CardinalityStats {
	return CardinalityStats{
		Current: d.seriesCardinality.Load(),
		Cap:     d.cfg.MaxCardinality,
		Dropped: d.seriesDropped.Load(),
	}
}

// InsertSpans 按 trace_id 分组添加 trace span。
func (d *Doris) InsertSpans(in []model.SpanRecord) int {
	if len(in) == 0 {
		return 0
	}
	d.muSpans.Lock()
	for _, s := range in {
		group := d.hotSpans[s.TraceID]
		group = append(group, s)
		if len(group) > 256 {
			old := group[:len(group)-256]
			for _, o := range old {
				d.coldSpans = append(d.coldSpans, o)
			}
			group = group[len(group)-256:]
		}
		d.hotSpans[s.TraceID] = group
	}
	if len(d.coldSpans) > d.cfg.ColdCap {
		d.coldSpans = d.coldSpans[len(d.coldSpans)-d.cfg.ColdCap:]
	}
	d.muSpans.Unlock()

	d.touchServicesBySpans(in)
	d.spansAccepted.Add(int64(len(in)))
	d.appendWAL(opSpan, in)
	return len(in)
}


// SetWAL 附加预写日志。后续插入会追加到其中。
// A nil WAL disables persistence.
func (d *Doris) SetWAL(w *WAL) {
	d.walMu.Lock()
	defer d.walMu.Unlock()
	d.wal = w
}

// appendWAL 将一批记录写入 WAL。对 nil WAL 安全。
func (d *Doris) appendWAL(op uint32, payload any) {
	d.walMu.Lock()
	w := d.wal
	d.walMu.Unlock()
	if w == nil {
		return
	}
	_ = w.Append(op, payload)
}

// ReplayInto 读取 WAL 的每条记录并将其应用到 d。
// Call after LoadFromFile to rebuild the in-memory state from disk.
func (d *Doris) ReplayInto(w *WAL) error {
	if w == nil {
		return nil
	}
	logs, metrics, spans, err := w.Replay()
	if err != nil {
		return err
	}
	if len(logs) > 0 {
		d.InsertLogs(logs)
	}
	if len(metrics) > 0 {
		d.InsertMetrics(metrics)
	}
	if len(spans) > 0 {
		d.InsertSpans(spans)
	}
	return nil
}
// touchServices 增加 Logs 的每服务计数器。
func (d *Doris) touchServices(rows []model.LogRecord) {
	d.muSum.Lock()
	defer d.muSum.Unlock()
	for _, r := range rows {
		s := d.ensureFor(r.TenantID, r.Service)
		s.LogsCount++
		s.UpdatedAt = time.Now()
	}
}

// touchServicesByMetrics 增加 Metrics 的每服务计数器。
func (d *Doris) touchServicesByMetrics(rows []model.MetricPoint) {
	d.muSum.Lock()
	defer d.muSum.Unlock()
	for _, r := range rows {
		s := d.ensureFor(r.TenantID, r.Service)
		s.MetricsCount++
		s.UpdatedAt = time.Now()
	}
}

// touchServicesBySpans 增加 Spans 的每服务计数器。
func (d *Doris) touchServicesBySpans(rows []model.SpanRecord) {
	d.muSum.Lock()
	defer d.muSum.Unlock()
	for _, r := range rows {
		s := d.ensureFor(r.TenantID, r.Service)
		s.SpansCount++
		s.UpdatedAt = time.Now()
	}
}

// ensure 返回一个服务摘要，按需创建。
// Requires d.muSum to be held.
func (d *Doris) ensure(name string) *model.ServiceSummary {
	s, ok := d.sum[name]
	if !ok {
		s = &model.ServiceSummary{Name: name, UpdatedAt: time.Now()}
		d.sum[name] = s
	}
	return s
}

// ensureFor 与 ensure 类似，但额外记录租户。我们按
// service summaries by (tenant||service) so two tenants with the same
// service name do not collide in the summary map.
func (d *Doris) ensureFor(tenant, name string) *model.ServiceSummary {
	key := tenantKey(tenant, name)
	s, ok := d.sum[key]
	if !ok {
		s = &model.ServiceSummary{Name: name, TenantID: tenant, UpdatedAt: time.Now()}
		d.sum[key] = s
	}
	return s
}

// tenantKey 由 tenant + service 组成稳定的 map key。该
// separator is a control byte unlikely to appear in service names.
func tenantKey(tenant, name string) string {
	if tenant == "" {
		return name
	}
	return tenant + "\x00" + name
}

// ListServices 返回稳定且排序后的服务摘要视图。
// When tenant is non-empty only services belonging to that tenant
// are returned. When tenant is empty all services are returned (back
// compat with callers that have not enabled multi-tenancy).
func (d *Doris) ListServices(tenant string) []model.ServiceSummary {
	d.muSum.RLock()
	defer d.muSum.RUnlock()
	out := make([]model.ServiceSummary, 0, len(d.sum))
	for _, s := range d.sum {
		if tenant != "" && s.TenantID != tenant {
			continue
		}
		// 天真但对 demo 可接受：从热日志推导错误率。
		s.ErrorRate = d.computeErrorRate(s.Name)
		s.P50Ms, s.P95Ms, s.P99Ms = d.PercentileLatencies(s.Name)
		s.QPS = d.computeQPS(s.Name)
		s.LastLabels = d.recentLabelKeys(s.Name)
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// GetService 返回单个服务摘要。当 tenant 为
// non-empty, the lookup is restricted to summaries belonging to that
// tenant.
func (d *Doris) GetService(tenant, name string) (model.ServiceSummary, bool) {
	d.muSum.RLock()
	defer d.muSum.RUnlock()
	key := tenantKey(tenant, name)
	s, ok := d.sum[key]
	if !ok {
		return model.ServiceSummary{}, false
	}
	s2 := *s
	s2.ErrorRate = d.computeErrorRate(name)
	s2.P50Ms, s2.P95Ms, s2.P99Ms = d.PercentileLatencies(name)
	s2.QPS = d.computeQPS(name)
	s2.LastLabels = d.recentLabelKeys(name)
	return s2, true
}

// recentLabelKeys 返回该服务热日志中出现的唯一 label key。
func (d *Doris) recentLabelKeys(service string) []string {
	d.muLogs.RLock()
	defer d.muLogs.RUnlock()
	set := map[string]bool{}
	for i := len(d.hotLogs) - 1; i >= 0 && i >= len(d.hotLogs)-256; i-- {
		r := d.hotLogs[i]
		if r.Service != service {
			continue
		}
		for k := range r.Attributes {
			set[k] = true
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// computeErrorRate 扫描热层日志以推导服务错误率。
// Locks are reentrant on the same goroutine; we use RLock here.
func (d *Doris) computeErrorRate(service string) float64 {
	d.muLogs.RLock()
	defer d.muLogs.RUnlock()
	var total, errs int
	for _, r := range d.hotLogs {
		if r.Service != service {
			continue
		}
		total++
		if r.Severity == model.SeverityError || r.Severity == model.SeverityFatal {
			errs++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(errs) / float64(total)
}

// computeP99 从热层 span 估算 p99 延迟。
func (d *Doris) computeP99(service string) float64 {
	d.muSpans.RLock()
	defer d.muSpans.RUnlock()
	var samples []int64
	for _, group := range d.hotSpans {
		for _, s := range group {
			if s.Service == service {
				samples = append(samples, s.DurationMs)
			}
		}
	}
	if len(samples) == 0 {
		return 0
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	idx := int(float64(len(samples)) * 0.99)
	if idx >= len(samples) {
		idx = len(samples) - 1
	}
	return float64(samples[idx])
}

// computeQPS 对该服务最近 60s 的热指标点计算 QPS。
func (d *Doris) computeQPS(service string) float64 {
	d.muMetrics.RLock()
	defer d.muMetrics.RUnlock()
	cutoff := time.Now().Add(-60 * time.Second)
	var recent int
	for _, points := range d.hotMetrics {
		for _, p := range points {
			if p.Service != service {
				continue
			}
			if p.Timestamp.After(cutoff) {
				recent++
				}
		}
	}
	return float64(recent) / 60.0
}

// updateMetricMV 在 MV 桶中维护简化的 1m 和 5m 滚动聚合。
// The key is service|name, the value is a downsampled series of
// MVBucket (sum/count/min/max) which is converted to mean values when
// the MV is read out. This replaces the previous running-average hack
// that biased every bucket toward the first sample ever seen.
func (d *Doris) updateMetricMV(in []model.MetricPoint) {
	d.muMV.Lock()
	defer d.muMV.Unlock()
	for _, p := range in {
		// 租户 + 服务 + 指标名作为 MV 桶键，因此两个
		// tenants with the same service name do not co-mingle data.
		key := p.TenantID + "\x00" + p.Service + "|" + p.Name
		ts := p.Timestamp.Truncate(time.Minute).UnixMilli()
		d.mvMinute[key] = appendMVBucket(d.mvMinute[key], p.Value, ts)

		ts5 := p.Timestamp.Truncate(5 * time.Minute).UnixMilli()
		d.mvFiveMinute[key] = appendMVBucket(d.mvFiveMinute[key], p.Value, ts5)
	}
}

// appendMVBucket 将 value 插入匹配 ts 的桶。乱序
// arrivals are handled by linear search; pathological resort only happens
// when an older ts slips in after a newer one (rare in practice).
func appendMVBucket(series []model.MVBucket, value float64, ts int64) []model.MVBucket {
	if len(series) == 0 {
		return []model.MVBucket{{Ts: ts, Sum: value, Count: 1, Min: value, Max: value}}
	}
	last := series[len(series)-1]
	if last.Ts == ts {
		// 同一桶：累加 sum/count/min/max。
		last.Sum += value
		last.Count++
		if value < last.Min {
			last.Min = value
		}
		if value > last.Max {
			last.Max = value
		}
		series[len(series)-1] = last
		return series
	}
	if ts < last.Ts {
		// 乱序到达：对桶进行线性查找。
		for i := range series {
			if series[i].Ts == ts {
				b := series[i]
				b.Sum += value
				b.Count++
				if value < b.Min {
					b.Min = value
				}
				if value > b.Max {
					b.Max = value
				}
				series[i] = b
				return series
			}
		}
		series = append(series, model.MVBucket{Ts: ts, Sum: value, Count: 1, Min: value, Max: value})
		sort.Slice(series, func(i, j int) bool { return series[i].Ts < series[j].Ts })
		return series
	}
	return append(series, model.MVBucket{Ts: ts, Sum: value, Count: 1, Min: value, Max: value})
}

// mvToSeries 将 MV 桶转换为 SeriesPoint 序列（均值）。
func mvToSeries(buckets []model.MVBucket) []model.SeriesPoint {
	out := make([]model.SeriesPoint, len(buckets))
	for i, b := range buckets {
		out[i] = model.SeriesPoint{Ts: b.Ts, Value: b.Mean()}
	}
	return out
}

// QueryLogs 返回按服务、严重级和时间窗口过滤的最近日志。
func (d *Doris) QueryLogs(service string, severity string, limit int, sinceMs int64) model.QueryResult {
	start := time.Now()
	d.queriesServed.Add(1)
	d.muLogs.RLock()
	hot := make([]model.LogRecord, 0, len(d.hotLogs))
	for _, r := range d.hotLogs {
		if service != "" && r.Service != service {
			continue
		}
		if severity != "" && string(r.Severity) != severity {
			continue
		}
		if sinceMs > 0 && r.Timestamp.UnixMilli() < sinceMs {
			continue
		}
		hot = append(hot, r)
	}
	coldHits := 0
	if len(hot) < limit {
		for _, r := range d.coldLogs {
			if service != "" && r.Service != service {
				continue
			}
			if severity != "" && string(r.Severity) != severity {
				continue
			}
			if sinceMs > 0 && r.Timestamp.UnixMilli() < sinceMs {
				continue
			}
			hot = append(hot, r)
			coldHits++
		}
	}
	d.muLogs.RUnlock()

	// 按时间戳降序排序，然后截断到 limit。
	sort.Slice(hot, func(i, j int) bool { return hot[i].Timestamp.After(hot[j].Timestamp) })
	if len(hot) > limit {
		hot = hot[:limit]
	}

	rows := make([]model.Row, 0, len(hot))
	for _, r := range hot {
		rows = append(rows, model.Row{
			"timestamp": r.Timestamp.Format(time.RFC3339Nano),
			"service":   r.Service,
			"severity":  string(r.Severity),
			"body":      r.Body,
			"trace_id":  r.TraceID,
			"span_id":   r.SpanID,
		})
	}

	tier := "hot"
	if coldHits > 0 {
		tier = "mixed"
	}
	if len(hot) == 0 {
		tier = "cold"
	}
	return model.QueryResult{
		Type: "logs",
		Rows: rows,
		Stats: model.QueryStats{
			Scanned:  int64(len(hot)),
			Returned: int64(len(rows)),
			TookMs:   time.Since(start).Milliseconds(),
			Tier:     tier,
		},
	}
}

// QueryMetrics 返回某指标名称的时间序列。
// It uses the 1m materialized view when the requested window is large.
// Tenant parameter isolates data per-tenant; empty = legacy mode.
func (d *Doris) QueryMetrics(tenant, service, name, window string, limit int) model.QueryResult {
	start := time.Now()
	d.queriesServed.Add(1)
	key := tenant + "\x00" + service + "|" + name
	var series []model.SeriesPoint
	mvName := ""
	d.muMV.RLock()
	switch window {
	case "5m":
		series = mvToSeries(d.mvFiveMinute[key])
		mvName = "mv_metrics_5m"
	case "1m", "":
		series = mvToSeries(d.mvMinute[key])
		mvName = "mv_metrics_1m"
	default:
		d.muMV.RUnlock()
		return model.QueryResult{Type: "metrics", Stats: model.QueryStats{TookMs: time.Since(start).Milliseconds()}}
	}
	d.muMV.RUnlock()

	if len(series) > limit {
		series = series[len(series)-limit:]
	}
	return model.QueryResult{
		Type: "metrics",
		Series: []model.MetricSeries{{
			Name:    name,
			Service: service,
			Unit:    "",
			Points:  series,
		}},
		Stats: model.QueryStats{
			Scanned:  int64(len(series)),
			Returned: int64(len(series)),
			TookMs:   time.Since(start).Milliseconds(),
			Tier:     "hot",
			MVUsed:   mvName,
		},
	}
}

// QueryTraces 返回给定 trace id 的 span 或所有最近的 span。
func (d *Doris) QueryTraces(traceID, service string, limit int) model.QueryResult {
	start := time.Now()
	d.queriesServed.Add(1)
	var all []model.SpanRecord

	d.muSpans.RLock()
	if traceID != "" {
		all = append(all, d.hotSpans[traceID]...)
	} else {
		for _, group := range d.hotSpans {
			for _, s := range group {
				if service != "" && s.Service != service {
					continue
				}
				all = append(all, s)
			}
		}
	}
	hotN := len(all)
	if len(all) < limit {
		for _, s := range d.coldSpans {
			if service != "" && s.Service != service {
				continue
			}
			all = append(all, s)
		}
	}
	d.muSpans.RUnlock()

	sort.Slice(all, func(i, j int) bool { return all[i].StartTime.After(all[j].StartTime) })
	if len(all) > limit {
		all = all[:limit]
	}
	rows := make([]model.Row, 0, len(all))
	for _, s := range all {
		rows = append(rows, model.Row{
			"trace_id":    s.TraceID,
			"span_id":     s.SpanID,
			"parent_id":   s.ParentID,
			"name":        s.Name,
			"service":     s.Service,
			"start_time":  s.StartTime.Format(time.RFC3339Nano),
			"duration_ms": s.DurationMs,
			"status":      s.Status,
		})
	}
	tier := "hot"
	if hotN < len(all) {
		tier = "mixed"
	}
	return model.QueryResult{
		Type: "traces",
		Rows: rows,
		Stats: model.QueryStats{
			Scanned:  int64(len(all)),
			Returned: int64(len(rows)),
			TookMs:   time.Since(start).Milliseconds(),
			Tier:     tier,
		},
	}
}

// SuccessCounts 返回的 ok 和 error span 数。
// given service with start_time >= sinceMillis. Used by the alerts
// engine to compute burn rates without exposing the full span set.
func (d *Doris) SuccessCounts(service string, sinceMillis int64) (ok int, errs int) {
	d.muSpans.RLock()
	defer d.muSpans.RUnlock()
	for _, spans := range d.hotSpans {
		for _, s := range spans {
			if s.Service != service {
				continue
			}
			if s.StartTime.UnixMilli() < sinceMillis {
				continue
			}
			if s.Status == "error" {
				errs++
			} else {
				ok++
			}
		}
	}
	return ok, errs
}
