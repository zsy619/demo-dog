// Package buffer implements a thread-safe, bounded batch buffer for the
// three observability signals. The exporter drains the buffer on a
// configurable cadence; any signal that does not fit the next batch is
// kept in place so nothing is dropped on backpressure.
//
// The buffer depends only on its own local types so it can be imported by
// any package, including the parent otlp package, without forming an
// import cycle.
package buffer

import (
	"sync"
	"time"
)

// LogRecord is the local representation of a log entry. The otlp package
// owns the wire JSON schema and converts these fields to its public
// LogRecord before marshaling.
type LogRecord struct {
	Timestamp  time.Time
	TenantID   string
	Service    string
	Severity   string
	Body       string
	Attributes map[string]string
	TraceID    string
	SpanID     string
}

// MetricPoint is the local metric representation.
type MetricPoint struct {
	Timestamp time.Time
	TenantID  string
	Service   string
	Name      string
	Value     float64
	Unit      string
	Type      string
	Labels    map[string]string

	// Histogram fields — populated when Type == "histogram" and the SDK
	// was configured with explicit bucket boundaries via
	// WithHistogramBuckets. BucketCounts and BucketBounds must have the
	// same length, ascending upper bounds, with the last entry being
	// +Inf (overflow). When these are zero the backend falls back to
	// the per-sample histogram path.
	BucketBounds   []float64
	BucketCounts   []int64
	HistogramCount int64
	HistogramSum   float64
	HistogramMin   float64
	HistogramMax   float64
}

// SpanRecord is the local span representation.
type SpanRecord struct {
	TraceID    string
	SpanID     string
	ParentID   string
	Name       string
	TenantID   string
	Service    string
	StartTime  time.Time
	DurationMs int64
	Status     string
	Attributes map[string]string
}

// Request is the local envelope. The otlp package marshals this into the
// wire JSON.
type Request struct {
	ResourceAttrs map[string]string
	Logs          []LogRecord
	Metrics       []MetricPoint
	Spans         []SpanRecord
}

// Buffer holds pending signal records and emits them as Request payloads.
// Methods are safe for concurrent use from any goroutine.
type Buffer struct {
	service  string
	resource map[string]string

	mu      sync.Mutex
	logs    []LogRecord
	metrics []MetricPoint
	spans   []SpanRecord
}

// New returns a buffer tied to the given service name and resource attrs.
func New(service string, resource map[string]string) *Buffer {
	return &Buffer{service: service, resource: resource}
}

// PushLog enqueues a log record.
func (b *Buffer) PushLog(l LogRecord) {
	if l.Service == "" {
		l.Service = b.service
	}
	if l.Timestamp.IsZero() {
		l.Timestamp = time.Now()
	}
	b.mu.Lock()
	b.logs = append(b.logs, l)
	b.mu.Unlock()
}

// PushMetric enqueues a metric point.
func (b *Buffer) PushMetric(m MetricPoint) {
	if m.Service == "" {
		m.Service = b.service
	}
	if m.Timestamp.IsZero() {
		m.Timestamp = time.Now()
	}
	b.mu.Lock()
	b.metrics = append(b.metrics, m)
	b.mu.Unlock()
}

// PushSpan enqueues a span.
func (b *Buffer) PushSpan(s SpanRecord) {
	if s.Service == "" {
		s.Service = b.service
	}
	if s.StartTime.IsZero() {
		s.StartTime = time.Now()
	}
	b.mu.Lock()
	b.spans = append(b.spans, s)
	b.mu.Unlock()
}

// Drain returns a snapshot Request and clears the pending records.
func (b *Buffer) Drain() Request {
	b.mu.Lock()
	defer b.mu.Unlock()

	req := Request{
		ResourceAttrs: b.resource,
		Logs:          b.logs,
		Metrics:       b.metrics,
		Spans:         b.spans,
	}
	b.logs = nil
	b.metrics = nil
	b.spans = nil
	return req
}

// Snapshot returns a copy without clearing.
func (b *Buffer) Snapshot() Request {
	b.mu.Lock()
	defer b.mu.Unlock()
	req := Request{
		ResourceAttrs: b.resource,
		Logs:          append([]LogRecord(nil), b.logs...),
		Metrics:       append([]MetricPoint(nil), b.metrics...),
		Spans:         append([]SpanRecord(nil), b.spans...),
	}
	return req
}

// Size returns the count of pending records.
func (b *Buffer) Size() (logs, metrics, spans int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.logs), len(b.metrics), len(b.spans)
}
