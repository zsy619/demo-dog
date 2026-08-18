// Package model defines unified data models for the three observability pillars.
//
// All observability signals are stored in-memory as LogRecord / MetricPoint /
// SpanRecord through the Store, which writes to an "In-Memory Doris" engine.
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

// Severity mirrors OTel LogSeverity (subset).
type Severity string

const (
	SeverityTrace Severity = "TRACE"
	SeverityDebug Severity = "DEBUG"
	SeverityInfo  Severity = "INFO"
	SeverityWarn  Severity = "WARN"
	SeverityError Severity = "ERROR"
	SeverityFatal Severity = "FATAL"
)

// SeverityRank returns 0..5 for TRACE..FATAL. Unknown severities rank below TRACE.
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

// LogRecord is a single log entry. OTLP AnyValue is simplified to string.
type LogRecord struct {
	Timestamp  time.Time         `json:"timestamp"`
	Service    string            `json:"service"`
	Severity   Severity          `json:"severity"`
	Body       string            `json:"body"`
	Attributes map[string]string `json:"attributes,omitempty"`
	TraceID    string            `json:"trace_id,omitempty"`
	SpanID     string            `json:"span_id,omitempty"`
}

// MetricPoint is a single metric data point, simplified OTel NumberDataPoint.
type MetricPoint struct {
	Timestamp time.Time         `json:"timestamp"`
	Service   string            `json:"service"`
	Name      string            `json:"name"`
	Value     float64           `json:"value"`
	Unit      string            `json:"unit,omitempty"`
	Type      string            `json:"type"` // gauge|counter|histogram
	Labels    map[string]string `json:"labels,omitempty"`

	// Histogram fields — only set when Type == "histogram". When the
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

// SpanRecord is a simplified OTel Span.
type SpanRecord struct {
	TraceID    string            `json:"trace_id"`
	SpanID     string            `json:"span_id"`
	ParentID   string            `json:"parent_id,omitempty"`
	Name       string            `json:"name"`
	Service    string            `json:"service"`
	StartTime  time.Time         `json:"start_time"`
	DurationMs int64             `json:"duration_ms"`
	Status     string            `json:"status"` // ok|error|unset
	Attributes map[string]string `json:"attributes,omitempty"`
}

// OTLPRequest is a JSON-simplified OTLP-style write payload.
// Real OTLP uses Protobuf; this demo uses JSON but keeps OTel naming.
type OTLPRequest struct {
	ResourceAttrs map[string]string `json:"resource_attrs"`
	Logs          []LogRecord       `json:"logs,omitempty"`
	Metrics       []MetricPoint     `json:"metrics,omitempty"`
	Spans         []SpanRecord      `json:"spans,omitempty"`
}

// OTLPResponse is the write acknowledgement, with retry hints per signal.
type OTLPResponse struct {
	AcceptedLogs    int      `json:"accepted_logs"`
	AcceptedMetrics int      `json:"accepted_metrics"`
	AcceptedSpans   int      `json:"accepted_spans"`
	RetryLogs       int      `json:"retry_logs"`
	RetryMetrics    int      `json:"retry_metrics"`
	RetrySpans      int      `json:"retry_spans"`
	Errors          []string `json:"errors,omitempty"`
}

// ServiceSummary is per-service overview consumed by the frontend cards.
type ServiceSummary struct {
	Name         string    `json:"name"`
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

// SeriesPoint is a single point on a time series.
type SeriesPoint struct {
	Ts    int64   `json:"ts"` // ms
	Value float64 `json:"value"`
}

// MVBucket is a single time-bucketed aggregate. Each bucket represents
// a 1- or 5-minute window and stores sum+count so we can compute a
// proper mean when the bucket is read out (rather than the previous
// "running average" hack that biased toward the first sample).
//
// On rollover (older buckets evicted to keep MV bounded), callers can
// compute min/max in addition to the mean by reading the partially
// populated fields.
type MVBucket struct {
	Ts    int64   `json:"ts"`    // bucket start, ms
	Sum   float64 `json:"sum"`   // sum of values in the window
	Count int64   `json:"count"` // number of samples
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
}

// Mean returns the bucket mean (0 if empty).
func (b MVBucket) Mean() float64 {
	if b.Count == 0 {
		return 0
	}
	return b.Sum / float64(b.Count)
}

// HistogramView is the read-out of an aggregated OTel histogram. The
// Bounds slice is the upper bound of each bucket (exclusive) with the
// last entry representing +Inf overflow. Counts are the per-bucket
// counts since the series began. Total/Sum/Min/Max are running totals
// across the lifetime of the series.
type HistogramView struct {
	Bounds []float64 `json:"bounds"`
	Counts []int64   `json:"counts"`
	Total  int64     `json:"total"`
	Sum    float64   `json:"sum"`
	Min    float64   `json:"min"`
	Max    float64   `json:"max"`
}

// MetricSeries is a labeled time series for frontend charts.
type MetricSeries struct {
	Name    string        `json:"name"`
	Service string        `json:"service"`
	Unit    string        `json:"unit"`
	Labels  map[string]string `json:"labels,omitempty"`
	Points  []SeriesPoint `json:"points"`
}

// QueryResult is the generic query response.
type QueryResult struct {
	Type   string         `json:"type"` // logs|metrics|traces
	Rows   []Row          `json:"rows"`
	Series []MetricSeries `json:"series,omitempty"`
	Stats  QueryStats     `json:"stats"`
}

// Row is a generic columnar row rendered by the frontend.
type Row map[string]any

// QueryStats reports query engine statistics.
//
// Fields:
//   - Scanned: total rows touched in the in-memory table
//   - Returned: rows actually returned to the caller
//   - TookMs:  query wall-clock latency
//   - Tier:    storage tier that served the query (hot | cold)
//   - MVUsed:  materialized view name if any
type QueryStats struct {
	Scanned  int64  `json:"scanned"`
	Returned int64  `json:"returned"`
	TookMs   int64  `json:"took_ms"`
	Tier     string `json:"tier"`
	MVUsed   string `json:"mv_used,omitempty"`
}

// LabelKeys returns the set of attribute keys that have been observed
// across all stored records. Useful for building the "filter by label"
// dropdown in the frontend.
type LabelKeysResponse struct {
	Logs    []string `json:"logs"`
	Metrics []string `json:"metrics"`
	Spans   []string `json:"spans"`
}

// ServiceMapEdge represents one edge in the service dependency graph.
type ServiceMapEdge struct {
	From   string `json:"from"` // caller / parent
	To     string `json:"to"`   // callee / child
	Calls  int64  `json:"calls"`
	Errors int64  `json:"errors"`
	AvgMs  float64 `json:"avg_ms"`
	P99Ms  float64 `json:"p99_ms"`
}

// ServiceMap is the response for /api/service-map.
type ServiceMap struct {
	Edges []ServiceMapEdge `json:"edges"`
	Nodes []string         `json:"nodes"` // distinct services in the map
}

// ServiceDetail bundles the per-service drill-down payload for /api/services/{name}/detail.
// It surfaces top endpoints (span-name histogram), recent errors, recent trace IDs, and
// the per-metric time series window so the frontend can render a complete service overview
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

// EndpointStats aggregates span activity for one endpoint / span name.
type EndpointStats struct {
	Name   string  `json:"name"`
	Count  int64   `json:"count"`
	Errors int64   `json:"errors"`
	AvgMs  float64 `json:"avg_ms"`
	P99Ms  float64 `json:"p99_ms"`
}
