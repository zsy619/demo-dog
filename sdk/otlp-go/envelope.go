// Package otlpgo provides conversion between the OpenTelemetry
// Collector's wire format (OTLP) and the demo-dog in-process model.
//
// The OpenTelemetry collector already speaks OTLP/HTTP on
// /v1/{logs,metrics,traces}. This package is for users who embed
// demo-dog directly into a custom otelcol receiver or processor.
//
// Conversion is best-effort: extra fields are ignored, missing fields
// are defaulted. The wire is lossless round-trip.
package otlpgo

import "time"

// Severity matches OTel's SeverityNumber enum.
type Severity int

const (
	SeverityTrace Severity = iota + 1
	SeverityDebug
	SeverityInfo
	SeverityWarn
	SeverityError
	SeverityFatal
)

// ResourceAttrs is the canonical "resource attributes" map the
// collector stamps on every signal.
type ResourceAttrs map[string]string

// Span is the OTel span model.
type Span struct {
	TraceID    string
	SpanID     string
	ParentSpan string
	Name       string
	Start      time.Time
	End        time.Time
	Status     string
	Attrs      ResourceAttrs
}

// LogRecord is the OTel log model.
type LogRecord struct {
	Timestamp time.Time
	Severity  Severity
	Body      string
	TraceID   string
	SpanID    string
	Attrs     ResourceAttrs
}

// MetricPoint is the OTel metric point.
type MetricPoint struct {
	Timestamp time.Time
	Name      string
	Value     float64
	Attrs     ResourceAttrs
}

// Envelope is what an otelcol processor hands to demo-dog.
type Envelope struct {
	TenantID string
	Service  string
	Spans    []Span
	Logs     []LogRecord
	Metrics  []MetricPoint
}

// SpanDTO is the serialisable form for an outgoing span.
type SpanDTO struct {
	TenantID    string            `json:"tenant_id"`
	Service     string            `json:"service,omitempty"`
	TraceID     string            `json:"trace_id"`
	SpanID      string            `json:"span_id"`
	ParentSpanID string           `json:"parent_span_id,omitempty"`
	Name        string            `json:"name"`
	StartTime   time.Time         `json:"start_time"`
	DurationMs  int64             `json:"duration_ms"`
	Status      string            `json:"status"`
	Attributes  map[string]string `json:"attributes,omitempty"`
}

// LogDTO is the serialisable form for an outgoing log.
type LogDTO struct {
	TenantID   string            `json:"tenant_id"`
	Service    string            `json:"service"`
	Timestamp  time.Time         `json:"timestamp"`
	Severity   string            `json:"severity"`
	Body       string            `json:"body"`
	TraceID    string            `json:"trace_id,omitempty"`
	SpanID     string            `json:"span_id,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// MetricDTO is the serialisable form for an outgoing metric.
type MetricDTO struct {
	TenantID  string            `json:"tenant_id"`
	Service   string            `json:"service"`
	Timestamp time.Time         `json:"timestamp"`
	Name      string            `json:"name"`
	Value     float64           `json:"value"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// Bundle is the wire payload demo-dog accepts at /api/ingest/otlp.
type Bundle struct {
	TenantID string `json:"tenant_id,omitempty"`
	ResourceAttrs map[string]string `json:"resource_attrs,omitempty"`
	Logs    []LogDTO    `json:"logs,omitempty"`
	Metrics []MetricDTO `json:"metrics,omitempty"`
	Spans   []SpanDTO   `json:"spans,omitempty"`
}

// ToBundle converts an Envelope into the demo-dog wire bundle.
// The result is JSON-marshalable and can be POSTed to
// /api/ingest/otlp with no further transformation.
func (e Envelope) ToBundle() Bundle {
	tenant := e.TenantID
	if tenant == "" {
		tenant = "default"
	}
	service := e.Service
	if service == "" {
		service = "unknown"
	}
	b := Bundle{
		TenantID:     tenant,
		ResourceAttrs: map[string]string{"service.name": service},
	}
	for _, l := range e.Logs {
		b.Logs = append(b.Logs, LogDTO{
			TenantID:   tenant,
			Service:    service,
			Timestamp:  l.Timestamp,
			Severity:   severityToString(l.Severity),
			Body:       l.Body,
			TraceID:    l.TraceID,
			SpanID:     l.SpanID,
			Attributes: l.Attrs,
		})
	}
	for _, m := range e.Metrics {
		b.Metrics = append(b.Metrics, MetricDTO{
			TenantID:  tenant,
			Service:   service,
			Timestamp: m.Timestamp,
			Name:      m.Name,
			Value:     m.Value,
			Labels:    m.Attrs,
		})
	}
	for _, sp := range e.Spans {
		b.Spans = append(b.Spans, SpanDTO{
			TenantID:     tenant,
			Service:      service,
			TraceID:      sp.TraceID,
			SpanID:       sp.SpanID,
			ParentSpanID: sp.ParentSpan,
			Name:         sp.Name,
			StartTime:    sp.Start,
			DurationMs:   sp.End.Sub(sp.Start).Milliseconds(),
			Status:       sp.Status,
			Attributes:   sp.Attrs,
		})
	}
	return b
}

func severityToString(s Severity) string {
	switch s {
	case SeverityTrace, SeverityDebug:
		return "DEBUG"
	case SeverityInfo:
		return "INFO"
	case SeverityWarn:
		return "WARN"
	case SeverityError:
		return "ERROR"
	case SeverityFatal:
		return "FATAL"
	}
	return "INFO"
}
