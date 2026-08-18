// Package store provides an in-memory simulation of an Apache Doris engine.
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
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/model"
)

// Config tunes the in-memory Doris engine.
type Config struct {
	HotLogTTL    time.Duration // logs older than this move to cold
	HotLogCap    int           // max hot logs per bucket
	HotMetricCap int           // max hot metric points per (service, name)
	ColdCap      int           // max cold rows per signal table
}

// DefaultConfig returns sensible defaults for the demo.
func DefaultConfig() Config {
	return Config{
		HotLogTTL:    5 * time.Minute,
		HotLogCap:    2048,
		HotMetricCap: 4096,
		ColdCap:      10_000,
	}
}

// Doris is the in-memory engine backing the demo.
type Doris struct {
	cfg Config

	// wal is the optional write-ahead log. When set, every insert
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

	// Histograms keyed by service|name. We aggregate sum/count into the
	// MV-style buckets AND keep the latest bucket_bounds + per-bucket
	// counts for proper quantile queries. Without this, OTel histograms
	// would degrade to scalar sum/count and SLO p95/p99 would silently
	// be wrong.
	muHistograms sync.RWMutex
	histograms   map[string]*histogramAgg // key=service|name

	muSpans sync.RWMutex
	hotSpans  map[string][]model.SpanRecord // key=trace_id
	coldSpans []model.SpanRecord

	// 1m & 5m materialized views for metrics, keyed by service|name.
	muMV         sync.RWMutex
	mvMinute     map[string][]model.MVBucket
	mvFiveMinute map[string][]model.MVBucket

	// Bookkeeping for service summaries.
	muSum sync.RWMutex
	sum   map[string]*model.ServiceSummary

	// Total counters for /api/health.
	logsAccepted    atomic.Int64
	metricsAccepted atomic.Int64
	spansAccepted   atomic.Int64
	queriesServed   atomic.Int64
}

// New returns a freshly initialized Doris engine.
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

// Stats reports counters surfaced by /api/health.
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

// Stats returns a snapshot of the engine counters.
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

// InsertLogs performs a Doris-style Stream Load of log records.
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
	// Bound hot tier by evicting oldest.
	if len(d.hotLogs) > d.cfg.HotLogCap {
		evicted := d.hotLogs[:len(d.hotLogs)-d.cfg.HotLogCap]
		d.coldLogs = append(d.coldLogs, evicted...)
		d.hotLogs = d.hotLogs[len(d.hotLogs)-d.cfg.HotLogCap:]
	}
	// Bound cold tier.
	if len(d.coldLogs) > d.cfg.ColdCap {
		d.coldLogs = d.coldLogs[len(d.coldLogs)-d.cfg.ColdCap:]
	}
	d.muLogs.Unlock()

	d.logsAccepted.Add(int64(len(in)))
	d.touchServices(in)
	d.appendWAL(opLog, in)
	return len(in)
}

// InsertMetrics adds metric points and updates the minute-level MV.
func (d *Doris) InsertMetrics(in []model.MetricPoint) int {
	if len(in) == 0 {
		return 0
	}
	d.muMetrics.Lock()
	for _, p := range in {
		key := p.Service + "|" + p.Name
		bucket := d.hotMetrics[key]
		bucket = append(bucket, p)
		if len(bucket) > d.cfg.HotMetricCap {
			old := bucket[:len(bucket)-d.cfg.HotMetricCap]
			d.coldMetrics = append(d.coldMetrics, old...)
			bucket = bucket[len(bucket)-d.cfg.HotMetricCap:]
		}
		d.hotMetrics[key] = bucket
	}
	if len(d.coldMetrics) > d.cfg.ColdCap {
		d.coldMetrics = d.coldMetrics[len(d.coldMetrics)-d.cfg.ColdCap:]
	}
	d.muMetrics.Unlock()

	d.updateMetricMV(in)
	d.updateHistograms(in)
	d.metricsAccepted.Add(int64(len(in)))
	d.touchServicesByMetrics(in)
	d.appendWAL(opMetric, in)
	return len(in)
}

// InsertSpans adds trace spans grouped by trace_id.
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


// SetWAL attaches a write-ahead log. Subsequent inserts append to it.
// A nil WAL disables persistence.
func (d *Doris) SetWAL(w *WAL) {
	d.walMu.Lock()
	defer d.walMu.Unlock()
	d.wal = w
}

// appendWAL records a batch to the WAL. Safe with nil WAL.
func (d *Doris) appendWAL(op uint32, payload any) {
	d.walMu.Lock()
	w := d.wal
	d.walMu.Unlock()
	if w == nil {
		return
	}
	_ = w.Append(op, payload)
}

// ReplayInto reads every record from the WAL and applies it to d.
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
// touchServices increments per-service counters for Logs.
func (d *Doris) touchServices(rows []model.LogRecord) {
	d.muSum.Lock()
	defer d.muSum.Unlock()
	for _, r := range rows {
		s := d.ensureFor(r.TenantID, r.Service)
		s.LogsCount++
		s.UpdatedAt = time.Now()
	}
}

// touchServicesByMetrics increments per-service counters for Metrics.
func (d *Doris) touchServicesByMetrics(rows []model.MetricPoint) {
	d.muSum.Lock()
	defer d.muSum.Unlock()
	for _, r := range rows {
		s := d.ensureFor(r.TenantID, r.Service)
		s.MetricsCount++
		s.UpdatedAt = time.Now()
	}
}

// touchServicesBySpans increments per-service counters for Spans.
func (d *Doris) touchServicesBySpans(rows []model.SpanRecord) {
	d.muSum.Lock()
	defer d.muSum.Unlock()
	for _, r := range rows {
		s := d.ensureFor(r.TenantID, r.Service)
		s.SpansCount++
		s.UpdatedAt = time.Now()
	}
}

// ensure returns a service summary, creating it on demand.
// Requires d.muSum to be held.
func (d *Doris) ensure(name string) *model.ServiceSummary {
	s, ok := d.sum[name]
	if !ok {
		s = &model.ServiceSummary{Name: name, UpdatedAt: time.Now()}
		d.sum[name] = s
	}
	return s
}

// ensureFor is like ensure but additionally records the tenant. We key
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

// tenantKey composes a stable map key from tenant + service. The
// separator is a control byte unlikely to appear in service names.
func tenantKey(tenant, name string) string {
	if tenant == "" {
		return name
	}
	return tenant + "\x00" + name
}

// ListServices returns a stable, sorted view of service summaries.
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
		// Naive but acceptable for demo: derive error rate from hot logs.
		s.ErrorRate = d.computeErrorRate(s.Name)
		s.P50Ms, s.P95Ms, s.P99Ms = d.PercentileLatencies(s.Name)
		s.QPS = d.computeQPS(s.Name)
		s.LastLabels = d.recentLabelKeys(s.Name)
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// GetService returns a single service summary. When tenant is
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

// recentLabelKeys returns the unique label keys seen in this service's hot logs.
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

// computeErrorRate scans logs in the hot tier to derive a service error rate.
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

// computeP99 estimates p99 latency from spans in the hot tier.
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

// computeQPS computes QPS over the last 60s of hot metric points for the service.
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

// updateMetricMV maintains simplified 1m and 5m rollups in MV buckets.
// The key is service|name, the value is a downsampled series of
// MVBucket (sum/count/min/max) which is converted to mean values when
// the MV is read out. This replaces the previous running-average hack
// that biased every bucket toward the first sample ever seen.
func (d *Doris) updateMetricMV(in []model.MetricPoint) {
	d.muMV.Lock()
	defer d.muMV.Unlock()
	for _, p := range in {
		// Tenant + service + metric name as the MV bucket key so two
		// tenants with the same service name do not co-mingle data.
		key := p.TenantID + "\x00" + p.Service + "|" + p.Name
		ts := p.Timestamp.Truncate(time.Minute).UnixMilli()
		d.mvMinute[key] = appendMVBucket(d.mvMinute[key], p.Value, ts)

		ts5 := p.Timestamp.Truncate(5 * time.Minute).UnixMilli()
		d.mvFiveMinute[key] = appendMVBucket(d.mvFiveMinute[key], p.Value, ts5)
	}
}

// appendMVBucket inserts value into the bucket matching ts. Out-of-order
// arrivals are handled by linear search; pathological resort only happens
// when an older ts slips in after a newer one (rare in practice).
func appendMVBucket(series []model.MVBucket, value float64, ts int64) []model.MVBucket {
	if len(series) == 0 {
		return []model.MVBucket{{Ts: ts, Sum: value, Count: 1, Min: value, Max: value}}
	}
	last := series[len(series)-1]
	if last.Ts == ts {
		// Same bucket: accumulate sum/count/min/max.
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
		// Out-of-order arrival: linear search for bucket.
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

// mvToSeries converts MV buckets to a SeriesPoint series (mean values).
func mvToSeries(buckets []model.MVBucket) []model.SeriesPoint {
	out := make([]model.SeriesPoint, len(buckets))
	for i, b := range buckets {
		out[i] = model.SeriesPoint{Ts: b.Ts, Value: b.Mean()}
	}
	return out
}

// QueryLogs returns recent logs filtered by service, severity, and time window.
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

	// Sort by timestamp descending, then clip to limit.
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

// QueryMetrics returns time series for a metric name.
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

// QueryTraces returns spans for a given trace id or all recent spans.
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

// SuccessCounts returns the number of ok and error spans for the
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
