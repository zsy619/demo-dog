// Package otlp is a pure-Go SDK that emits OTLP-style JSON payloads and
// ships them to a DOG collector. The wire format is JSON-simplified OTel.
package otlp

import (
	"encoding/json"
	"time"
)

// Severity mirrors the OTel severity enum.
type Severity string

const (
	SeverityTrace Severity = "TRACE"
	SeverityDebug Severity = "DEBUG"
	SeverityInfo  Severity = "INFO"
	SeverityWarn  Severity = "WARN"
	SeverityError Severity = "ERROR"
	SeverityFatal Severity = "FATAL"
)

// MetricType is the metric shape.
type MetricType string

const (
	TypeCounter   MetricType = "counter"
	TypeGauge     MetricType = "gauge"
	TypeHistogram MetricType = "histogram"
)

// SpanStatus mirrors OTel Span.Status code: ok, error, unset.
type SpanStatus string

const (
	StatusOK    SpanStatus = "ok"
	StatusError SpanStatus = "error"
	StatusUnset SpanStatus = "unset"
)

// LogRecord is one log entry. Matches backend model.LogRecord.
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

// MetricPoint is one number data point. Matches backend model.MetricPoint.
type MetricPoint struct {
	Timestamp time.Time         `json:"timestamp"`
	TenantID  string            `json:"tenant_id,omitempty"`
	Service   string            `json:"service"`
	Name      string            `json:"name"`
	Value     float64           `json:"value"`
	Unit      string            `json:"unit,omitempty"`
	Type      MetricType        `json:"type"`
	Labels    map[string]string `json:"labels,omitempty"`

	// OTel histogram fields. Populated by SDK users who configure
	// WithHistogramBuckets(...). When present and Type == TypeHistogram
	// the backend uses them to compute true quantiles. Optional for
	// backwards compatibility — older exporters that send only sum/count
	// still work, the backend falls back to per-sample aggregation.
	BucketBounds   []float64 `json:"bucket_bounds,omitempty"`
	BucketCounts   []int64   `json:"bucket_counts,omitempty"`
	HistogramCount int64     `json:"histogram_count,omitempty"`
	HistogramSum   float64   `json:"histogram_sum,omitempty"`
	HistogramMin   float64   `json:"histogram_min,omitempty"`
	HistogramMax   float64   `json:"histogram_max,omitempty"`
}

// SpanRecord is one span. Matches backend model.SpanRecord.
type SpanRecord struct {
	TraceID    string            `json:"trace_id"`
	SpanID     string            `json:"span_id"`
	ParentID   string            `json:"parent_id,omitempty"`
	Name       string            `json:"name"`
	TenantID   string            `json:"tenant_id,omitempty"`
	Service    string            `json:"service"`
	StartTime  time.Time         `json:"start_time"`
	DurationMs int64             `json:"duration_ms"`
	Status     SpanStatus        `json:"status"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Links      []SpanLink        `json:"links,omitempty"`
}

// SpanLink ties a span to another trace -- used for fan-in/out patterns
// or async work that crosses trace boundaries. Backend currently ignores
// this field for the simplified envelope; the standard OTel envelope
// emits it under "links".
type SpanLink struct {
	TraceID    string            `json:"trace_id"`
	SpanID     string            `json:"span_id"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// Request is the wire envelope. Matches backend model.OTLPRequest.
type Request struct {
	ResourceAttrs map[string]string `json:"resource_attrs"`
	Logs          []LogRecord       `json:"logs,omitempty"`
	Metrics       []MetricPoint     `json:"metrics,omitempty"`
	Spans         []SpanRecord      `json:"spans,omitempty"`
}

// Response mirrors backend model.OTLPResponse.
type Response struct {
	AcceptedLogs    int      `json:"accepted_logs"`
	AcceptedMetrics int      `json:"accepted_metrics"`
	AcceptedSpans   int      `json:"accepted_spans"`
	RetryLogs       int      `json:"retry_logs"`
	RetryMetrics    int      `json:"retry_metrics"`
	RetrySpans      int      `json:"retry_spans"`
	Errors          []string `json:"errors,omitempty"`
}

// String returns the JSON wire encoding of the request.
func (r *Request) String() string {
	b, _ := json.Marshal(r)
	return string(b)
}
