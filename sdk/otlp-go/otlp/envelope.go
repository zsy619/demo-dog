// OTel standard envelope encoding.
//
// The simplified JSON envelope (`{resource_attrs, logs, metrics, spans}`)
// is what this SDK emits by default -- it matches the demo-dog backend
// directly. But the standard OTLP/HTTP JSON wire format (`resourceSpans`,
// `resourceLogs`, `resourceMetrics` with nested `scopeSpans`/etc.) is
// what every off-the-shelf OTel collector understands. The exporter
// here produces that standard envelope so the SDK can also talk to
// vanilla OTel collectors without modifying your service code.
//
// Reference: https://opentelemetry.io/docs/specs/otlp/#json-protobuf-encoding
package otlp

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// OTelEnvelope is the standard OTLP/HTTP JSON envelope. We keep our
// internal types (LogRecord / MetricPoint / SpanRecord) and serialize
// them on the fly rather than maintain a separate set of structs.
type OTelEnvelope struct {
	ResourceSpans   []OTelResourceSpans   `json:"resourceSpans,omitempty"`
	ResourceMetrics []OTelResourceMetrics `json:"resourceMetrics,omitempty"`
	ResourceLogs    []OTelResourceLogs    `json:"resourceLogs,omitempty"`
}

type OTelResourceSpans struct {
	Resource   OTelResource      `json:"resource"`
	ScopeSpans []OTelScopeSpans `json:"scopeSpans"`
}

type OTelScopeSpans struct {
	Scope OTelScope  `json:"scope"`
	Spans []OTelSpan `json:"spans"`
}

type OTelSpan struct {
	TraceID           string     `json:"traceId"`
	SpanID            string     `json:"spanId"`
	ParentSpanID      string     `json:"parentSpanId,omitempty"`
	Name              string     `json:"name"`
	Kind              int        `json:"kind,omitempty"`
	StartTimeUnixNano string     `json:"startTimeUnixNano"`
	EndTimeUnixNano   string     `json:"endTimeUnixNano"`
	Attributes        []OTelAttr `json:"attributes,omitempty"`
	Status            OTelStatus `json:"status"`
	Links             []OTelLink `json:"links,omitempty"`
}

type OTelStatus struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
}

// OTelLink ties a span to another trace -- useful for fan-in/out patterns
// and asynchronous work. The simplified envelope carries links via the
// Links field on SpanRecord; here they are emitted verbatim.
type OTelLink struct {
	TraceID    string     `json:"traceId"`
	SpanID     string     `json:"spanId"`
	Attributes []OTelAttr `json:"attributes,omitempty"`
}

type OTelResourceMetrics struct {
	Resource     OTelResource       `json:"resource"`
	ScopeMetrics []OTelScopeMetrics `json:"scopeMetrics"`
}

type OTelScopeMetrics struct {
	Scope   OTelScope    `json:"scope"`
	Metrics []OTelMetric `json:"metrics"`
}

type OTelMetric struct {
	Name      string         `json:"name"`
	Unit      string         `json:"unit,omitempty"`
	Sum       *OTelSum       `json:"sum,omitempty"`
	Gauge     *OTelGauge     `json:"gauge,omitempty"`
	Histogram *OTelHistogram `json:"histogram,omitempty"`
}

type OTelSum struct {
	AggregationTemporality int            `json:"aggregationTemporality"`
	IsMonotonic            bool           `json:"isMonotonic"`
	DataPoints             []OTelNumberDP `json:"dataPoints"`
}

type OTelGauge struct {
	DataPoints []OTelNumberDP `json:"dataPoints"`
}

type OTelNumberDP struct {
	Attributes        []OTelAttr `json:"attributes,omitempty"`
	StartTimeUnixNano string     `json:"startTimeUnixNano"`
	TimeUnixNano      string     `json:"timeUnixNano"`
	AsInt             string     `json:"asInt,omitempty"`
	AsDouble          float64    `json:"asDouble"`
}

type OTelHistogram struct {
	AggregationTemporality int                `json:"aggregationTemporality"`
	DataPoints             []OTelHistogramDP `json:"dataPoints"`
}

type OTelHistogramDP struct {
	Attributes        []OTelAttr  `json:"attributes,omitempty"`
	StartTimeUnixNano string      `json:"startTimeUnixNano"`
	TimeUnixNano      string      `json:"timeUnixNano"`
	Count             string      `json:"count"`
	Sum               float64     `json:"sum"`
	Min               float64     `json:"min,omitempty"`
	Max               float64     `json:"max,omitempty"`
	Mean              float64     `json:"mean,omitempty"`
	BucketCounts      []string    `json:"bucketCounts,omitempty"`      // OTel expects string[] for u64
	ExplicitBounds    []float64   `json:"explicitBounds,omitempty"`     // ascending, last is +Inf
}

type OTelResourceLogs struct {
	Resource  OTelResource    `json:"resource"`
	ScopeLogs []OTelScopeLogs `json:"scopeLogs"`
}

type OTelScopeLogs struct {
	Scope      OTelScope       `json:"scope"`
	LogRecords []OTelLogRecord `json:"logRecords"`
}

type OTelLogRecord struct {
	TimeUnixNano         string     `json:"timeUnixNano"`
	ObservedTimeUnixNano string     `json:"observedTimeUnixNano,omitempty"`
	SeverityNumber       int        `json:"severityNumber,omitempty"`
	SeverityText         string     `json:"severityText,omitempty"`
	Body                 OTelAnyVal `json:"body"`
	Attributes           []OTelAttr `json:"attributes,omitempty"`
	TraceID              string     `json:"traceId,omitempty"`
	SpanID               string     `json:"spanId,omitempty"`
}

type OTelResource struct {
	Attributes []OTelAttr `json:"attributes"`
}

type OTelScope struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type OTelAttr struct {
	Key   string         `json:"key"`
	Value map[string]any `json:"value"`
}

// OTelAnyVal is the AnyValue shape -- {stringValue|intValue|doubleValue|boolValue}.
type OTelAnyVal map[string]any

// OTelString / OTelInt / OTelFloat / OTelBool build a body / attr value.
func OTelString(s string) OTelAnyVal  { return OTelAnyVal{"stringValue": s} }
func OTelInt(n int64) OTelAnyVal     { return OTelAnyVal{"intValue": strconv.FormatInt(n, 10)} }
func OTelFloat(f float64) OTelAnyVal { return OTelAnyVal{"doubleValue": f} }
func OTelBool(b bool) OTelAnyVal     { return OTelAnyVal{"boolValue": b} }

// OTelSeverityNumber maps the simplified severity text to OTel severity
// number scale (1..24). Unknown values fall back to 9 (INFO).
func OTelSeverityNumber(severity string) int {
	switch severity {
	case "TRACE":
		return 1
	case "DEBUG":
		return 5
	case "INFO":
		return 9
	case "WARN":
		return 13
	case "ERROR":
		return 17
	case "FATAL":
		return 21
	}
	return 9
}

// EncodeOTLPEnvelope converts our internal Request into the standard
// OTLP JSON envelope. The function is exported so SDK users building
// custom transports (e.g. a kafka exporter) can reuse the marshaling.
func EncodeOTLPEnvelope(req Request) ([]byte, error) {
	env := OTelEnvelope{
		ResourceSpans:   buildResourceSpans(req),
		ResourceMetrics: buildResourceMetrics(req),
		ResourceLogs:    buildResourceLogs(req),
	}
	return json.Marshal(env)
}

func buildResourceSpans(req Request) []OTelResourceSpans {
	if len(req.Spans) == 0 {
		return nil
	}
	return []OTelResourceSpans{{
		Resource: buildOTelResource(req.ResourceAttrs),
		ScopeSpans: []OTelScopeSpans{{
			Scope: OTelScope{Name: "otlp-go", Version: Version},
			Spans: mapSpans(req.Spans),
		}},
	}}
}

func buildResourceMetrics(req Request) []OTelResourceMetrics {
	if len(req.Metrics) == 0 {
		return nil
	}
	out := []OTelResourceMetrics{{
		Resource:     buildOTelResource(req.ResourceAttrs),
		ScopeMetrics: []OTelScopeMetrics{{Scope: OTelScope{Name: "otlp-go", Version: Version}}},
	}}
	for _, m := range req.Metrics {
		out[0].ScopeMetrics[0].Metrics = append(out[0].ScopeMetrics[0].Metrics, mapMetric(m))
	}
	return out
}

func buildResourceLogs(req Request) []OTelResourceLogs {
	if len(req.Logs) == 0 {
		return nil
	}
	out := []OTelResourceLogs{{
		Resource:  buildOTelResource(req.ResourceAttrs),
		ScopeLogs: []OTelScopeLogs{{Scope: OTelScope{Name: "otlp-go", Version: Version}}},
	}}
	for _, l := range req.Logs {
		out[0].ScopeLogs[0].LogRecords = append(out[0].ScopeLogs[0].LogRecords, mapLog(l))
	}
	return out
}

func buildOTelResource(resource map[string]string) OTelResource {
	attrs := make([]OTelAttr, 0, len(resource))
	for k, v := range resource {
		attrs = append(attrs, OTelAttr{Key: k, Value: map[string]any{"stringValue": v}})
	}
	return OTelResource{Attributes: attrs}
}

func mapSpans(in []SpanRecord) []OTelSpan {
	out := make([]OTelSpan, len(in))
	for i, s := range in {
		out[i] = OTelSpan{
			TraceID:           s.TraceID,
			SpanID:            s.SpanID,
			ParentSpanID:      s.ParentID,
			Name:              s.Name,
			Kind:              1, // INTERNAL
			StartTimeUnixNano: timeToUnixNano(s.StartTime),
			EndTimeUnixNano:   timeToUnixNano(s.StartTime.Add(time.Duration(s.DurationMs) * time.Millisecond)),
			Attributes:        mapAttrs(s.Attributes),
			Status:            mapStatus(s.Status),
			Links:             mapLinks(s.Links),
		}
	}
	return out
}

func mapLinks(in []SpanLink) []OTelLink {
	if len(in) == 0 {
		return nil
	}
	out := make([]OTelLink, len(in))
	for i, l := range in {
		out[i] = OTelLink{
			TraceID:    l.TraceID,
			SpanID:     l.SpanID,
			Attributes: mapAttrs(l.Attributes),
		}
	}
	return out
}

func mapMetric(m MetricPoint) OTelMetric {
	attrs := mapAttrs(m.Labels)
	ts := timeToUnixNano(m.Timestamp)
	start := ts
	switch m.Type {
	case TypeCounter:
		return OTelMetric{
			Name: m.Name,
			Unit: m.Unit,
			Sum: &OTelSum{
				AggregationTemporality: 2, // CUMULATIVE
				IsMonotonic:            true,
				DataPoints: []OTelNumberDP{{
					Attributes:        attrs,
					StartTimeUnixNano: start,
					TimeUnixNano:      ts,
					AsDouble:          m.Value,
				}},
			},
		}
	case TypeHistogram:
		dp := OTelHistogramDP{
			Attributes:        attrs,
			StartTimeUnixNano: start,
			TimeUnixNano:      ts,
			Count:             "1",
			Sum:               m.Value,
			Min:               m.Value,
			Max:               m.Value,
			Mean:              m.Value,
		}
		// When the SDK emitted explicit bucket boundaries, forward them
		// (and the bucket counts) to the backend so the receiver can
		// compute true quantiles instead of a per-sample approximation.
		if len(m.BucketBounds) > 0 {
			dp.ExplicitBounds = append([]float64(nil), m.BucketBounds...)
			counts := make([]string, len(m.BucketCounts))
			for i, c := range m.BucketCounts {
				counts[i] = strconv.FormatInt(c, 10)
			}
			dp.BucketCounts = counts
			// Sum/count/min/max override per-sample values.
			dp.Count = strconv.FormatInt(m.HistogramCount, 10)
			dp.Sum = m.HistogramSum
			dp.Min = m.HistogramMin
			dp.Max = m.HistogramMax
			if m.HistogramCount > 0 {
				dp.Mean = m.HistogramSum / float64(m.HistogramCount)
			}
		}
		return OTelMetric{
			Name: m.Name,
			Unit: m.Unit,
			Histogram: &OTelHistogram{
				AggregationTemporality: 1, // DELTA
				DataPoints:             []OTelHistogramDP{dp},
			},
		}
	default:
		return OTelMetric{
			Name: m.Name,
			Unit: m.Unit,
			Gauge: &OTelGauge{
				DataPoints: []OTelNumberDP{{
					Attributes:        attrs,
					StartTimeUnixNano: start,
					TimeUnixNano:      ts,
					AsDouble:          m.Value,
				}},
			},
		}
	}
}

func mapLog(l LogRecord) OTelLogRecord {
	return OTelLogRecord{
		TimeUnixNano:         timeToUnixNano(l.Timestamp),
		ObservedTimeUnixNano: timeToUnixNano(l.Timestamp),
		SeverityNumber:       OTelSeverityNumber(string(l.Severity)),
		SeverityText:         string(l.Severity),
		Body:                 OTelAnyVal{"stringValue": l.Body},
		Attributes:           mapAttrs(l.Attributes),
		TraceID:              l.TraceID,
		SpanID:               l.SpanID,
	}
}

func mapAttrs(in map[string]string) []OTelAttr {
	if len(in) == 0 {
		return nil
	}
	out := make([]OTelAttr, 0, len(in))
	for k, v := range in {
		out = append(out, OTelAttr{Key: k, Value: map[string]any{"stringValue": v}})
	}
	return out
}

func mapStatus(s SpanStatus) OTelStatus {
	switch s {
	case StatusOK:
		return OTelStatus{Code: 1}
	case StatusError:
		return OTelStatus{Code: 2, Message: "error"}
	}
	return OTelStatus{Code: 0}
}

func timeToUnixNano(t time.Time) string {
	if t.IsZero() {
		t = time.Now()
	}
	return strconv.FormatInt(t.UnixNano(), 10)
}

// OTelExporter implements the standard envelope wire format. It is the
// right choice when the SDK should talk to a vanilla OTLP-aware
// collector rather than the demo-dog simplified ingest.
//
// Wire compatibility:
//   - POST {endpoint}/api/ingest/otlp-json
//   - Content-Type: application/json+otlp
//   - Body: standard OTel envelope
type OTelExporter struct {
	endpoint   string
	httpClient *http.Client
}

// OTelExporterOption mutates an OTelExporter at construction.
type OTelExporterOption func(*OTelExporter)

// WithOTelHTTPClient overrides the default *http.Client.
func WithOTelHTTPClient(c *http.Client) OTelExporterOption {
	return func(e *OTelExporter) {
		if c != nil {
			e.httpClient = c
		}
	}
}

// WithOTelEndpoint overrides the default ingest path.
func WithOTelEndpoint(url string) OTelExporterOption {
	return func(e *OTelExporter) {
		e.endpoint = url
	}
}

// NewOTelExporter builds an OTel envelope exporter pointing at the
// standard /api/ingest/otlp-json ingest endpoint on the demo-dog
// collector, or any other collector that speaks the OTel JSON envelope.
func NewOTelExporter(base string, opts ...OTelExporterOption) *OTelExporter {
	e := &OTelExporter{
		endpoint:   joinEndpoint(base, "/api/ingest/otlp-json"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Export marshals the request as the standard OTLP envelope and POSTs it.
func (e *OTelExporter) Export(ctx context.Context, req Request) (*Response, error) {
	body, err := EncodeOTLPEnvelope(req)
	if err != nil {
		return nil, fmt.Errorf("marshal otel envelope: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json+otlp")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("transport: %w", err)
	}
	defer resp.Body.Close()

	rb, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("otel collector rejected: status=%d body=%s", resp.StatusCode, string(rb))
	}

	var out Response
	if err := json.Unmarshal(rb, &out); err != nil {
		// Some collectors return an empty body on success; do not fail.
		out.AcceptedLogs = len(req.Logs)
		out.AcceptedMetrics = len(req.Metrics)
		out.AcceptedSpans = len(req.Spans)
	}
	return &out, nil
}

// randomTraceID / randomSpanID exposed for users that need to mint IDs
// outside the SDK (e.g. when constructing synthetic events).
func randomTraceID() string { return randomHexOtel(16) }
func randomSpanID() string  { return randomHexOtel(8) }

// randomHexOtel is a tiny helper used by the standard-envelope exporter
// and any downstream helper. Same logic as internal/transform.Hex but
// exposed here so envelope consumers do not need to import the internal
// package.
func randomHexOtel(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		for i := range b {
			b[i] = 0
		}
	}
	return hex.EncodeToString(b)
}
