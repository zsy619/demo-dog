// Package model 定义三大可观测性支柱的统一数据模型。
//
// All observability signals are stored in-memory as LogRecord / MetricPoint /
// SpanRecord through the Store, which 写入 to an "In-Memory Doris" engine.
// The naming follows Apache Doris / OTel conventions:
//
//   - logs: PK (service_name, ts_ms), hash bucketed
//   - metrics: PK (service_name, name, ts_ms), materialized views on 1m/5m windows
//   - traces: trace_id clustered, span_id ordered
//
// This lets the public API mimic Doris Stream Load / SELECT semantics for
// demonstrating hot/cold tiering.
package model

import "time"

// Severity 对应 OTel LogSeverity（子集）。
type Severity string

const (
	SeverityTrace Severity = "TRACE"
	SeverityDebug Severity = "DEBUG"
	SeverityInfo  Severity = "INFO"
	SeverityWarn  Severity = "WARN"
	SeverityError Severity = "ERROR"
	SeverityFatal Severity = "FATAL"
)

// SeverityRank 对 TRACE..FATAL 返回 0..5。未知严重级排在 TRACE 之下。
func (s Severity) Rank() int {
	switch s {
	case SeverityTrace:
		return 0
	case SeverityDebug:
		return 1
	case SeverityInfo:
		return 2
	case SeverityWarn:
		return 3
	case SeverityError:
		return 4
	case SeverityFatal:
		return 5
	}
	return -1
}

// LogRecord 是一条日志记录。OTLP AnyValue 被简化为 string。
type LogRecord struct {
	Timestamp  time.Time         `json:"timestamp"`
	TenantID   string            `json:"tenant_id,omitempty"`
	Service    string            `json:"service"`
	Severity   Severity          `json:"severity"`
	Body       string            `json:"body"`
	Attributes map[string]string `json:"attributes,omitempty"`
	TraceID    string            `json:"trace_id,omitempty"`
	SpanID     string            `json:"span_id,omitempty"`
}

// MetricPoint 是单个指标数据点，是简化的 OTel NumberDataPoint。
type MetricPoint struct {
	Timestamp time.Time         `json:"timestamp"`
	TenantID  string            `json:"tenant_id,omitempty"`
	Service   string            `json:"service"`
	Name      string            `json:"name"`
	Value     float64           `json:"value"`
	Unit      string            `json:"unit,omitempty"`
	Type      string            `json:"type"` // gauge|counter|histogram
	Labels    map[string]string `json:"labels,omitempty"`

	// 直方图字段——仅当 Type 为 histogram 时设置。当
	// exporter supplies explicit bucket boundaries we keep them so the
	// store can compute true quantiles instead of falling back to a
	// log-bucketed approximation.
	BucketBounds   []float64 `json:"bucket_bounds,omitempty"`   // upper bounds (exclusive), ascending, last entry is overflow (+Inf)
	BucketCounts   []int64   `json:"bucket_counts,omitempty"`   // count per bucket (including +Inf overflow)
	HistogramCount int64     `json:"histogram_count,omitempty"` // total count (== sum of bucket counts)
	HistogramSum   float64   `json:"histogram_sum,omitempty"`   // sum of all observations
	HistogramMin   float64   `json:"histogram_min,omitempty"`
	HistogramMax   float64   `json:"histogram_max,omitempty"`
}

// SpanRecord 是简化的 OTel Span。
type SpanRecord struct {
	TraceID    string            `json:"trace_id"`
	SpanID     string            `json:"span_id"`
	ParentID   string            `json:"parent_id,omitempty"`
	Name       string            `json:"name"`
	TenantID   string            `json:"tenant_id,omitempty"`
	Service    string            `json:"service"`
	StartTime  time.Time         `json:"start_time"`
	DurationMs int64             `json:"duration_ms"`
	Status     string            `json:"status"` // ok|error|unset
	Attributes map[string]string `json:"attributes,omitempty"`
}

// OTLPRequest 是 JSON 简化的 OTLP 风格写入负载。
// Real OTLP uses Protobuf; this demo uses JSON but keeps OTel naming.
type OTLPRequest struct {
	TenantID      string            `json:"tenant_id,omitempty"`
	ResourceAttrs map[string]string `json:"resource_attrs"`
	Logs          []LogRecord       `json:"logs,omitempty"`
	Metrics       []MetricPoint     `json:"metrics,omitempty"`
	Spans         []SpanRecord      `json:"spans,omitempty"`
}

// OTLPResponse 是写入确认，含每个 signal 的重试提示。
type OTLPResponse struct {
	AcceptedLogs    int      `json:"accepted_logs"`
	AcceptedMetrics int      `json:"accepted_metrics"`
	AcceptedSpans   int      `json:"accepted_spans"`
	RetryLogs       int      `json:"retry_logs"`
	RetryMetrics    int      `json:"retry_metrics"`
	RetrySpans      int      `json:"retry_spans"`
	Errors          []string `json:"errors,omitempty"`
}

// ServiceSummary 是前端卡片使用的每服务概览。
type ServiceSummary struct {
	Name         string    `json:"name"`
	TenantID     string    `json:"tenant_id,omitempty"`
	LogsCount    int64     `json:"logs_count"`
	MetricsCount int64     `json:"metrics_count"`
	SpansCount   int64     `json:"spans_count"`
	ErrorRate    float64   `json:"error_rate"`
	P99Ms        float64   `json:"p99_ms"`
	P95Ms        float64   `json:"p95_ms"`
	P50Ms        float64   `json:"p50_ms"`
	QPS          float64   `json:"qps"`
	UpdatedAt    time.Time `json:"updated_at"`
	LastLabels   []string  `json:"last_labels,omitempty"`
}

// SeriesPoint 是时间序列上的一个点。
type SeriesPoint struct {
	Ts    int64   `json:"ts"` // ms
	Value float64 `json:"value"`
}

// MVBucket 是单个时间桶聚合。每个桶代表
// a 1- or 5-minute window and stores sum+count so we can compute a
// proper mean when the bucket is read out (rather than 前一个
// "running average" hack that biased toward the first sample).
//
// On rollover (older buckets evicted to keep 物化视图 bounded), callers can
// compute min/max in addition to the mean by reading the partially
// populated fields.
type MVBucket struct {
	Ts    int64   `json:"ts"`    // bucket start, ms
	Sum   float64 `json:"sum"`   // sum of values in the window
	Count int64   `json:"count"` // number of samples
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
}

// Mean 返回桶均值（空时为 0）。
func (b MVBucket) Mean() float64 {
	if b.Count == 0 {
		return 0
	}
	return b.Sum / float64(b.Count)
}

// HistogramView 是聚合 OTel 直方图的读出。
// Bounds slice is the upper bound of each bucket (exclusive) with the
// last entry representing +Inf overflow. Counts are the per-bucket
// counts since the 序列 began. Total/Sum/Min/Max are running totals
// across the lifetime of the 序列.
type HistogramView struct {
	Bounds []float64 `json:"bounds"`
	Counts []int64   `json:"counts"`
	Total  int64     `json:"total"`
	Sum    float64   `json:"sum"`
	Min    float64   `json:"min"`
	Max    float64   `json:"max"`
}

// MetricSeries 是用于前端图表的标签时间序列。
type MetricSeries struct {
	Name    string        `json:"name"`
	Service string        `json:"service"`
	Unit    string        `json:"unit"`
	Labels  map[string]string `json:"labels,omitempty"`
	Points  []SeriesPoint `json:"points"`
}

// QueryResult 是通用查询响应。
type QueryResult struct {
	Type   string         `json:"type"` // logs|metrics|traces
	Rows   []Row          `json:"rows"`
	Series []MetricSeries `json:"series,omitempty"`
	Stats  QueryStats     `json:"stats"`
}

// Row 是前端渲染的通用列式行。
type Row map[string]any

// QueryStats 报告查询引擎统计。
//
// Fields:
//   - Scanned: total rows touched in the in-memory table
//   - Returned: rows actually returned to the caller
//   - TookMs:  query wall-clock latency
//   - Tier:    storage tier that served the query (hot | cold)
//   - MVUsed:  物化视图 name if any
type QueryStats struct {
	Scanned  int64  `json:"scanned"`
	Returned int64  `json:"returned"`
	TookMs   int64  `json:"took_ms"`
	Tier     string `json:"tier"`
	MVUsed   string `json:"mv_used,omitempty"`
}

// LabelKeys 返回已观察到的属性 key 集合
// across all stored 记录. Useful for building the "filter by label"
// dropdown in the frontend.
type LabelKeysResponse struct {
	Logs    []string `json:"logs"`
	Metrics []string `json:"metrics"`
	Spans   []string `json:"spans"`
}

// ServiceMapEdge 表示服务依赖图中的一条边。
type ServiceMapEdge struct {
	From   string `json:"from"` // caller / parent
	To     string `json:"to"`   // callee / child
	Calls  int64  `json:"calls"`
	Errors int64  `json:"errors"`
	AvgMs  float64 `json:"avg_ms"`
	P99Ms  float64 `json:"p99_ms"`
}

// ServiceMap 是 /api/service-map 的响应。
type ServiceMap struct {
	Edges []ServiceMapEdge `json:"edges"`
	Nodes []string         `json:"nodes"` // distinct services in the map
}

// ServiceDetail 打包 /api/services/{name}/detail 的每服务下钻负载。
// It surfaces top endpoints (span-name 直方图), recent errors, recent trace IDs, and
// the per-metric 时序 window so the frontend can render a complete service overview
// page with a single round-trip.
type ServiceDetail struct {
	Summary      ServiceSummary  `json:"summary"`
	TopOps       []EndpointStats `json:"top_ops"`
	MetricNames  []string        `json:"metric_names"`
	RecentErrors []LogRecord     `json:"recent_errors"`
	RecentTraces []string        `json:"recent_traces"`
	Endpoints    []EndpointStats `json:"endpoints"`
	QPS          []SeriesPoint   `json:"qps"`
}

// EndpointStats 聚合一个 endpoint / span name 的 span 活动。
type EndpointStats struct {
	Name   string  `json:"name"`
	Count  int64   `json:"count"`
	Errors int64   `json:"errors"`
	AvgMs  float64 `json:"avg_ms"`
	P99Ms  float64 `json:"p99_ms"`
}
