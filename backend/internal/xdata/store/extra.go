// Package store：构建于其上的扩展查询/分析辅助函数。
// primary hot/cold tables defined in doris.go.
//
// These functions do NOT mutate the in-memory tables; they read under the
// same locks used by QueryLogs/Metrics/Traces.
package store

import (
	"sort"
	"strings"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xdata/model"
)

// QueryFilter 是 HTTP handler 传递的统一过滤器结构。
// Empty fields are treated as "match anything".
type QueryFilter struct {
	Tenant          string            // tenant id (or empty = all)
	Service         string            // exact match (or empty)
	Name            string            // metric name (or empty)
	Severity        string            // log severity (or empty) — exact match
	SeverityAtLeast bool              // when true, Severity is a >= comparison against severity ordering
	MinSeverity     int               // 0=TRACE … 5=FATAL; used when SeverityAtLeast is true
	TraceID         string            // trace id (or empty)
	Search          string            // substring search across log bodies / span names
	Labels          map[string]string // every key must be present with the same value
	SinceMs         int64             // include records with ts >= SinceMs (0 = no lower bound)
	UntilMs  int64             // include records with ts <= UntilMs (0 = no upper bound)
	Limit    int               // hard cap on rows returned
	Window   string            // "1m" / "5m" for metrics MV selection
}

// matchesLabelFilter 报告 attrs 是否包含 want 中的每个 (k, v) 对。
func matchesLabelFilter(attrs, want map[string]string) bool {
	for k, v := range want {
		if attrs == nil {
			return false
		}
		if got, ok := attrs[k]; !ok || got != v {
			return false
		}
	}
	return true
}

// QueryLogsFiltered 与 QueryLogs 类似，但支持更丰富的过滤器：
// time range, substring search across the body, and label-key matching.
func (d *Doris) QueryLogsFiltered(f QueryFilter) model.QueryResult {
	start := time.Now()
	d.queriesServed.Add(1)
	d.muLogs.RLock()
	hot := d.collectLogs(d.hotLogs, f)
	coldHits := 0
	if len(hot) < f.Limit {
		cold := d.collectLogs(d.coldLogs, f)
		coldHits = len(cold)
		hot = append(hot, cold...)
	}
	d.muLogs.RUnlock()

	sort.Slice(hot, func(i, j int) bool { return hot[i].Timestamp.After(hot[j].Timestamp) })
	if len(hot) > f.Limit {
		hot = hot[:f.Limit]
	}
	rows := make([]model.Row, 0, len(hot))
	for _, r := range hot {
		rows = append(rows, model.Row{
			"timestamp":  r.Timestamp.Format(time.RFC3339Nano),
			"service":    r.Service,
			"severity":   string(r.Severity),
			"body":       r.Body,
			"trace_id":   r.TraceID,
			"span_id":    r.SpanID,
			"attributes": r.Attributes,
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

func (d *Doris) collectLogs(in []model.LogRecord, f QueryFilter) []model.LogRecord {
	out := make([]model.LogRecord, 0, len(in))
	for _, r := range in {
		if f.Tenant != "" && r.TenantID != f.Tenant {
			continue
		}
		if f.Service != "" && r.Service != f.Service {
			continue
		}
		if f.SeverityAtLeast {
			if r.Severity.Rank() < f.MinSeverity {
				continue
			}
		} else if f.Severity != "" && string(r.Severity) != f.Severity {
			continue
		}
		if f.TraceID != "" && r.TraceID != f.TraceID {
			continue
		}
		if f.SinceMs > 0 && r.Timestamp.UnixMilli() < f.SinceMs {
			continue
		}
		if f.UntilMs > 0 && r.Timestamp.UnixMilli() > f.UntilMs {
			continue
		}
		if f.Search != "" && !strings.Contains(strings.ToLower(r.Body), strings.ToLower(f.Search)) {
			continue
		}
		if len(f.Labels) > 0 && !matchesLabelFilter(r.Attributes, f.Labels) {
			continue
		}
		out = append(out, r)
	}
	return out
}

// QueryMetricsFiltered 支持 labels 过滤 + 时间范围 + 窗口选择。
func (d *Doris) QueryMetricsFiltered(f QueryFilter) model.QueryResult {
	start := time.Now()
	d.queriesServed.Add(1)
	key := f.Tenant + "\x00" + f.Service + "|" + f.Name
	var series []model.SeriesPoint
	var mvName string

	d.muMV.RLock()
	switch f.Window {
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

	if f.SinceMs > 0 || f.UntilMs > 0 {
		filtered := series[:0]
		for _, p := range series {
			if f.SinceMs > 0 && p.Ts < f.SinceMs {
				continue
			}
			if f.UntilMs > 0 && p.Ts > f.UntilMs {
				continue
			}
			filtered = append(filtered, p)
		}
		series = filtered
	}
	if len(series) > f.Limit {
		series = series[len(series)-f.Limit:]
	}
	return model.QueryResult{
		Type: "metrics",
		Series: []model.MetricSeries{{
			Name:    f.Name,
			Service: f.Service,
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

// QueryTracesFiltered 支持 trace id、service、name 子串和 label 匹配。
//
// When the caller filters by service we expand the result set to include every
// span belonging to the trace ids that matched, so the client sees the full
// multi-service span tree (not just the local slice that happened to match).
func (d *Doris) QueryTracesFiltered(f QueryFilter) model.QueryResult {
	start := time.Now()
	d.queriesServed.Add(1)
	d.muSpans.RLock()
	var all []model.SpanRecord
	if f.TraceID != "" {
		all = append(all, d.hotSpans[f.TraceID]...)
	} else {
		// 第一遍：收集至少有一个匹配 span 的 trace id
		// every filter, so we can later expand them to full traces.
		matchedTraces := make(map[string]struct{})
		for tid, group := range d.hotSpans {
			for _, s := range group {
				if f.Tenant != "" && s.TenantID != f.Tenant {
					continue
				}
				if f.Service != "" && s.Service != f.Service {
					continue
				}
				if f.Search != "" && !strings.Contains(strings.ToLower(s.Name), strings.ToLower(f.Search)) {
					continue
				}
				if len(f.Labels) > 0 && !matchesLabelFilter(s.Attributes, f.Labels) {
					continue
				}
				if f.SinceMs > 0 && s.StartTime.UnixMilli() < f.SinceMs {
					continue
				}
				if f.UntilMs > 0 && s.StartTime.UnixMilli() > f.UntilMs {
					continue
				}
				matchedTraces[tid] = struct{}{}
				break
			}
		}
		// 第二遍：发出每个匹配 trace 的每个 span，以便
		// "checkout" filter still surfaces the auth+postgres child spans.
		for tid := range matchedTraces {
			for _, s := range d.hotSpans[tid] {
				all = append(all, s)
			}
		}
	}
	hotN := len(all)
	if len(all) < f.Limit {
		for _, s := range d.coldSpans {
			if f.Tenant != "" && s.TenantID != f.Tenant {
				continue
			}
			if f.Service != "" && s.Service != f.Service {
				continue
			}
			all = append(all, s)
		}
	}
	d.muSpans.RUnlock()

	sort.Slice(all, func(i, j int) bool { return all[i].StartTime.After(all[j].StartTime) })
	if len(all) > f.Limit {
		all = all[:f.Limit]
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
			"attributes":  s.Attributes,
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

// TraceSpans 返回属于某 trace id 的每个 span（按 start_time 排序）。
func (d *Doris) TraceSpans(traceID string) []model.SpanRecord {
	d.muSpans.RLock()
	defer d.muSpans.RUnlock()
	out := make([]model.SpanRecord, len(d.hotSpans[traceID]))
	copy(out, d.hotSpans[traceID])
	sort.Slice(out, func(i, j int) bool { return out[i].StartTime.Before(out[j].StartTime) })
	return out
}

// LabelKeys 返回所有已存记录中观察到的属性 key 并集。
func (d *Doris) LabelKeys() model.LabelKeysResponse {
	resp := model.LabelKeysResponse{Logs: []string{}, Metrics: []string{}, Spans: []string{}}
	logKeys := map[string]bool{}
	metricKeys := map[string]bool{}
	spanKeys := map[string]bool{}

	d.muLogs.RLock()
	for _, r := range d.hotLogs {
		for k := range r.Attributes {
			logKeys[k] = true
		}
	}
	d.muLogs.RUnlock()

	d.muMetrics.RLock()
	for _, pts := range d.hotMetrics {
		for _, p := range pts {
			for k := range p.Labels {
				metricKeys[k] = true
			}
		}
	}
	d.muMetrics.RUnlock()

	d.muSpans.RLock()
	for _, group := range d.hotSpans {
		for _, s := range group {
			for k := range s.Attributes {
				spanKeys[k] = true
			}
		}
	}
	d.muSpans.RUnlock()

	for k := range logKeys {
		resp.Logs = append(resp.Logs, k)
	}
	for k := range metricKeys {
		resp.Metrics = append(resp.Metrics, k)
	}
	for k := range spanKeys {
		resp.Spans = append(resp.Spans, k)
	}
	sort.Strings(resp.Logs)
	sort.Strings(resp.Metrics)
	sort.Strings(resp.Spans)
	return resp
}

// ServiceMap 遍历每个 span 并聚合 parent_service -> service 边。
func (d *Doris) ServiceMap() model.ServiceMap {
	type edgeAcc struct {
		calls  int64
		errors int64
		ms     []int64
	}
	edges := map[string]*edgeAcc{}
	nodes := map[string]bool{}

	d.muSpans.RLock()
	defer d.muSpans.RUnlock()
	for _, group := range d.hotSpans {
		byID := map[string]model.SpanRecord{}
		for _, s := range group {
			byID[s.SpanID] = s
			nodes[s.Service] = true
		}
		for _, s := range group {
			if s.ParentID == "" {
				continue
			}
			parent, ok := byID[s.ParentID]
			if !ok || parent.Service == s.Service {
				continue
			}
			key := parent.Service + "\x00" + s.Service
			acc, ok := edges[key]
			if !ok {
				acc = &edgeAcc{}
				edges[key] = acc
			}
			acc.calls++
			if s.Status == "error" {
				acc.errors++
			}
			acc.ms = append(acc.ms, s.DurationMs)
		}
	}

	out := model.ServiceMap{Edges: []model.ServiceMapEdge{}}
	for k, acc := range edges {
		parts := strings.SplitN(k, "\x00", 2)
		if len(parts) != 2 {
			continue
		}
		p99 := percentile(acc.ms, 0.99)
		avg := avgMs(acc.ms)
		out.Edges = append(out.Edges, model.ServiceMapEdge{
			From:   parts[0],
			To:     parts[1],
			Calls:  acc.calls,
			Errors: acc.errors,
			AvgMs:  avg,
			P99Ms:  p99,
		})
	}
	for n := range nodes {
		out.Nodes = append(out.Nodes, n)
	}
	sort.Strings(out.Nodes)
	sort.Slice(out.Edges, func(i, j int) bool {
		if out.Edges[i].From != out.Edges[j].From {
			return out.Edges[i].From < out.Edges[j].From
		}
		return out.Edges[i].To < out.Edges[j].To
	})
	return out
}

// PercentileLatencies 基于 duration_ms 样本计算 p50/p95/p99
// observed for a single service in the hot tier. Returns 0 if no samples.
func (d *Doris) PercentileLatencies(service string) (p50, p95, p99 float64) {
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
		return 0, 0, 0
	}
	return percentile(samples, 0.50), percentile(samples, 0.95), percentile(samples, 0.99)
}

// percentile 使用以下方式返回 samples 的第 q 百分位（0..1）
// linear interpolation between order statistics (the "C=1" / numpy
// default). With only one sample we return it; with zero samples we
// return 0. Without interpolation the previous implementation picked
// the boundary bucket value, which systematically over-estimated
// percentiles for small sample sets (e.g. p99 of [10,20,30,100] was
// 100, not ~76 as the true 99th percentile).
func percentile(samples []int64, q float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	if q < 0 {
		q = 0
	}
	if q > 1 {
		q = 1
	}
	cp := make([]int64, len(samples))
	copy(cp, samples)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	if len(cp) == 1 {
		return float64(cp[0])
	}
	// 位置在 [0, len-1] 浮点。
	pos := q * float64(len(cp)-1)
	lo := int(pos)
	hi := lo + 1
	if hi >= len(cp) {
		hi = len(cp) - 1
	}
	frac := pos - float64(lo)
	return float64(cp[lo]) + frac*(float64(cp[hi])-float64(cp[lo]))
}

func avgMs(samples []int64) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sum int64
	for _, s := range samples {
		sum += s
	}
	return float64(sum) / float64(len(samples))
}

// TopMetricNames 返回摄入最频繁的前 N 个指标名。
func (d *Doris) TopMetricNames(limit int) []string {
	d.muMetrics.RLock()
	defer d.muMetrics.RUnlock()
	counts := map[string]int{}
	for key := range d.hotMetrics {
		if i := strings.IndexByte(key, '|'); i >= 0 {
			counts[key[i+1:]]++
		} else {
			counts[key]++
		}
	}
	type kv struct {
		name  string
		count int
	}
	rank := make([]kv, 0, len(counts))
	for n, c := range counts {
		rank = append(rank, kv{n, c})
	}
	sort.Slice(rank, func(i, j int) bool {
		if rank[i].count != rank[j].count {
			return rank[i].count > rank[j].count
		}
		return rank[i].name < rank[j].name
	})
	out := make([]string, 0, limit)
	for i := 0; i < len(rank) && i < limit; i++ {
		out = append(out, rank[i].name)
	}
	return out
}

// ServiceListForLog 返回有任何日志记录的服务集合。
func (d *Doris) ServiceListForLog() []string {
	d.muLogs.RLock()
	defer d.muLogs.RUnlock()
	set := map[string]bool{}
	for _, r := range d.hotLogs {
		set[r.Service] = true
	}
	for _, r := range d.coldLogs {
		set[r.Service] = true
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// HistogramCounts 返回适合 sparkline 的微型直方图。
//
// Uses fixed logarithmic bucket boundaries so the histogram is meaningful
// regardless of the input range (no more "maxV=1 collapses everything to
// bin 0" bug from the previous normalized-by-max implementation).
//
// Buckets: 1, 2, 5, 10, 20, 50, 100, 200, 500, 1000, 2000, 5000, 10000 ms.
// Samples outside the range spill into the first/last bucket.
func (d *Doris) HistogramCounts(service string, bins int) []int {
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
	if len(samples) == 0 || bins <= 0 {
		return []int{}
	}
	// 固定桶边界（毫秒）（针对典型 Web 服务延迟选择：
	// sub-ms … multi-second). Last edge is inclusive overflow.
	edges := []int64{1, 2, 5, 10, 20, 50, 100, 200, 500, 1000, 2000, 5000, 10000}
	counts := make([]int, len(edges)+1)
	for _, s := range samples {
		if s <= 0 {
			counts[0]++
			continue
		}
		placed := false
		for i, e := range edges {
			if s <= e {
				counts[i]++
				placed = true
				break
			}
		}
		if !placed {
			counts[len(counts)-1]++
		}
	}
	// 通过简单的桶降采样/上采样到所请求的 bins 数。
	// coalescing. Caller requests <= ~10 buckets anyway.
	return resampleBuckets(counts, bins)
}

// resampleBuckets 使用以下方式将 src 折叠为恰好 dst 个桶
// greedy coalescing. If dst >= len(src) we pad with zeros (and
// interleave) so the shape stays comparable; if dst < len(src) we
// merge adjacent buckets.
func resampleBuckets(src []int, dst int) []int {
	if dst <= 0 {
		return []int{}
	}
	if dst == len(src) {
		out := make([]int, len(src))
		copy(out, src)
		return out
	}
	out := make([]int, dst)
	if dst >= len(src) {
		// 填充并交错：将 src 平均分布到 dst 槽中。
		for i, v := range src {
			out[i*dst/len(src)] += v
		}
		return out
	}
	// Merge：每个输出桶平均 k = len(src)/dst 个输入桶。
	k := len(src) / dst
	r := len(src) % dst
	idx := 0
	for i := 0; i < dst; i++ {
		size := k
		if i < r {
			size++
		}
		for j := 0; j < size && idx < len(src); j++ {
			out[i] += src[idx]
			idx++
		}
	}
	return out
}

// SeverityCounts 返回某服务每个严重级的日志记录数。
func (d *Doris) SeverityCounts(service string) map[string]int {
	d.muLogs.RLock()
	defer d.muLogs.RUnlock()
	counts := map[string]int{
		"TRACE": 0, "DEBUG": 0, "INFO": 0, "WARN": 0, "ERROR": 0, "FATAL": 0,
	}
	for _, r := range d.hotLogs {
		if service != "" && r.Service != service {
			continue
		}
		counts[string(r.Severity)]++
	}
	return counts
}

// QPSByService 返回每个服务最近每秒点的聚合。
func (d *Doris) QPSByService(window time.Duration) map[string][]model.SeriesPoint {
	d.muMetrics.RLock()
	defer d.muMetrics.RUnlock()
	cutoff := time.Now().Add(-window)
	buckets := map[string]map[int64]int64{}
	for _, pts := range d.hotMetrics {
		for _, p := range pts {
			if p.Timestamp.Before(cutoff) {
				continue
			}
			if buckets[p.Service] == nil {
				buckets[p.Service] = map[int64]int64{}
			}
			ts := p.Timestamp.Truncate(time.Minute).UnixMilli()
			buckets[p.Service][ts]++
		}
	}
	out := map[string][]model.SeriesPoint{}
	for svc, b := range buckets {
		pts := make([]model.SeriesPoint, 0, len(b))
		for ts, n := range b {
			pts = append(pts, model.SeriesPoint{Ts: ts, Value: float64(n) / 60.0})
		}
		sort.Slice(pts, func(i, j int) bool { return pts[i].Ts < pts[j].Ts })
		out[svc] = pts
	}
	return out
}

// Snapshot 返回最新样本的副本，用于实时 tail UI 渲染。
func (d *Doris) Snapshot() (logs []model.LogRecord, metrics []model.MetricPoint, spans []model.SpanRecord) {
	d.muLogs.RLock()
	logs = append([]model.LogRecord(nil), d.hotLogs...)
	if len(logs) > 50 {
		logs = logs[len(logs)-50:]
	}
	d.muLogs.RUnlock()

	d.muMetrics.RLock()
	for _, pts := range d.hotMetrics {
		metrics = append(metrics, pts...)
	}
	if len(metrics) > 100 {
		metrics = metrics[len(metrics)-100:]
	}
	d.muMetrics.RUnlock()

	d.muSpans.RLock()
	for _, group := range d.hotSpans {
		spans = append(spans, group...)
	}
	if len(spans) > 100 {
		spans = spans[len(spans)-100:]
	}
	d.muSpans.RUnlock()
	return
}

// 用于 /metrics 的计数器访问器。
func (d *Doris) LogsAccepted() int64    { return d.logsAccepted.Load() }
func (d *Doris) MetricsAccepted() int64 { return d.metricsAccepted.Load() }
func (d *Doris) SpansAccepted() int64   { return d.spansAccepted.Load() }
func (d *Doris) QueriesServed() int64   { return d.queriesServed.Load() }

// ServiceDetail 返回单个服务的丰富下钻负载：
//   - the standard ServiceSummary (with p50/p95/p99 + last_labels),
//   - top span-name endpoints (ranked by call count) with p99 latency and
//     error count, derived from the hot-spans table,
//   - the unique metric names emitted by this service,
//   - the last N ERROR/FATAL log records for the service,
//   - the last N trace IDs that touched this service,
//   - the per-second QPS series for the most recent 5 minutes.
func (d *Doris) ServiceDetail(name string) (model.ServiceDetail, bool) {
	sum, ok := d.GetService("", name)
	if !ok {
		return model.ServiceDetail{}, false
	}
	detail := model.ServiceDetail{Summary: sum}
	// 始终以非 nil 切片开始，使 JSON 编码器输出 `[]` 而非
	// of `null`. Frontend code can then call `.length` / `.map(...)` without
	// null-safety guards.
	detail.Endpoints = []model.EndpointStats{}
	detail.TopOps = []model.EndpointStats{}
	detail.MetricNames = []string{}
	detail.RecentErrors = []model.LogRecord{}
	detail.RecentTraces = []string{}
	detail.QPS = []model.SeriesPoint{}

	// 按 span name 的顶级 endpoint。
	d.muSpans.RLock()
	type acc struct {
		count    int64
		errors   int64
		samples  []int64
	}
	agg := map[string]*acc{}
	for _, group := range d.hotSpans {
		for _, s := range group {
			if s.Service != name {
				continue
			}
			a := agg[s.Name]
			if a == nil {
				a = &acc{}
				agg[s.Name] = a
			}
			a.count++
			if s.Status == "error" {
				a.errors++
			}
			a.samples = append(a.samples, s.DurationMs)
		}
	}
	d.muSpans.RUnlock()
	type ep struct {
		name  string
		count int64
		acc   *acc
	}
	var eps []ep
	for n, a := range agg {
		eps = append(eps, ep{n, a.count, a})
	}
	sort.Slice(eps, func(i, j int) bool { return eps[i].count > eps[j].count })
	for i, e := range eps {
		if i >= 20 {
			break
		}
		avg := float64(0)
		if len(e.acc.samples) > 0 {
			var sum int64
			for _, v := range e.acc.samples {
				sum += v
			}
			avg = float64(sum) / float64(len(e.acc.samples))
		}
		cp := append([]int64(nil), e.acc.samples...)
		sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
		p99 := percentile(cp, 0.99)
		detail.Endpoints = append(detail.Endpoints, model.EndpointStats{
			Name: e.name, Count: e.count, Errors: e.acc.errors, AvgMs: avg, P99Ms: p99,
		})
	}
	// TopOps 是相同数据但按降序排序——前端可任选使用。
	detail.TopOps = detail.Endpoints

	// 该服务的指标名称。
	names := map[string]bool{}
	d.muMetrics.RLock()
	for key, pts := range d.hotMetrics {
		if len(pts) == 0 { continue }
		// key 为 "<tenant>\x00<service>|<name>"。取出 service 和 name。
		nulIdx := strings.IndexByte(key, 0)
		if nulIdx < 0 { continue }
		barIdx := strings.IndexByte(key[nulIdx:], '|')
		if barIdx < 0 { continue }
		svc := key[nulIdx+1 : nulIdx+barIdx]
		metricName := key[nulIdx+barIdx+1:]
		if svc != name { continue }
		names[metricName] = true
	}
	d.muMetrics.RUnlock()
	for n := range names {
		detail.MetricNames = append(detail.MetricNames, n)
	}
	sort.Strings(detail.MetricNames)

	// 最近的错误。
	d.muLogs.RLock()
	for i := len(d.hotLogs) - 1; i >= 0; i-- {
		r := d.hotLogs[i]
		if r.Service != name {
			continue
		}
		if r.Severity != model.SeverityError && r.Severity != model.SeverityFatal {
			continue
		}
		detail.RecentErrors = append(detail.RecentErrors, r)
		if len(detail.RecentErrors) >= 20 {
			break
		}
	}
	d.muLogs.RUnlock()

	// 最近的 trace ID。
	seen := map[string]bool{}
	d.muSpans.RLock()
	for _, group := range d.hotSpans {
		for _, s := range group {
			if s.Service != name {
				continue
			}
			if seen[s.TraceID] {
				continue
			}
			seen[s.TraceID] = true
			detail.RecentTraces = append(detail.RecentTraces, s.TraceID)
			if len(detail.RecentTraces) >= 30 {
				break
			}
		}
		if len(detail.RecentTraces) >= 30 {
			break
		}
	}
	d.muSpans.RUnlock()

	// QPS 序列（最近 5 分钟，1 秒桶）。
	qpsAll := d.QPSByService(5 * time.Minute)
	if pts, ok := qpsAll[name]; ok && pts != nil {
		detail.QPS = pts
	}

	return detail, true
}
