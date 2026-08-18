// SDK facade. Exposes high-level Log / Counter / Gauge / Record / Trace
// helpers that translate to lower-level pushes into a thread-safe buffer.
// The buffer is drained by a background goroutine that calls Export on a
// configured cadence.
//
// Lifecycle:
//
//	sdk, err := otlp.New("http://localhost:18080",
//	    otlp.WithService("checkout"),
//	    otlp.WithServiceVersion("v1.0.0"),
//	    otlp.WithFlushInterval(2 * time.Second),
//	    otlp.WithMaxBatch(500),
//	)
//	if err != nil {
//	    return err
//	}
//	defer sdk.Shutdown(context.Background())
//
//	sdk.Log(ctx, otlp.SeverityInfo, "order placed",
//	    otlp.String("user_id", "u-42"))
//	sdk.Counter(ctx, "orders.placed", 1,
//	    otlp.String("channel", "web"))
//
// The Shutdown method flushes any pending records and stops the worker.
package otlp

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/zsy619/demo-dog/sdk/otlp-go/internal/buffer"
	"github.com/zsy619/demo-dog/sdk/otlp-go/internal/transform"
)

// Exporter is the minimal contract every SDK exporter must implement.
// Both *Exporter (simplified JSON) and *OTelExporter (standard envelope)
// satisfy this.
type ExporterInterface interface {
	Export(ctx context.Context, req Request) (*Response, error)
}

// SDK is the user-facing entry point. One SDK instance per service.
type SDK struct {
	exporter ExporterInterface
	buf      *buffer.Buffer
	resource map[string]string

	flushInterval time.Duration
	maxBatch      int
	sampler       Sampler
	errorHandler  func(err error)
	autoResource  bool

	stop chan struct{}
	done chan struct{}

	wg         sync.WaitGroup
	mu         sync.Mutex
	inShutdown bool

	stats Stats
}

// SDKOption configures the SDK at construction.
type SDKOption func(*SDK)

// WithService sets the service.name resource attribute. Required.
func WithService(name string) SDKOption {
	return func(s *SDK) {
		s.resource["service.name"] = name
	}
}

// WithServiceVersion sets service.version.
func WithServiceVersion(v string) SDKOption {
	return func(s *SDK) {
		s.resource["service.version"] = v
	}
}

// WithDeploymentEnvironment sets deployment.environment.
func WithDeploymentEnvironment(env string) SDKOption {
	return func(s *SDK) {
		s.resource["deployment.environment"] = env
	}
}

// WithHostName sets host.name.
func WithHostName(host string) SDKOption {
	return func(s *SDK) {
		s.resource["host.name"] = host
	}
}

// WithResourceAttrs merges extra resource attributes (e.g. telemetry.sdk.*).
func WithResourceAttrs(attrs map[string]string) SDKOption {
	return func(s *SDK) {
		for k, v := range attrs {
			s.resource[k] = v
		}
	}
}

// WithFlushInterval configures the background flush cadence. Default 2s.
func WithFlushInterval(d time.Duration) SDKOption {
	return func(s *SDK) {
		if d > 0 {
			s.flushInterval = d
		}
	}
}

// WithMaxBatch caps the number of records exported per flush. Default 500.
func WithMaxBatch(n int) SDKOption {
	return func(s *SDK) {
		if n > 0 {
			s.maxBatch = n
		}
	}
}

// WithExporter overrides the default Exporter. Both *Exporter (simplified
// JSON, default) and *OTelExporter (standard envelope) implement the
// ExporterInterface, so they can be passed here directly.
func WithExporter(e ExporterInterface) SDKOption {
	return func(s *SDK) {
		if e != nil {
			s.exporter = e
		}
	}
}

// WithSampler installs a custom Sampler. The default is AlwaysOnSampler.
// Sampler decisions are recorded in Stats.SamplerSkipped.
func WithSampler(smp Sampler) SDKOption {
	return func(s *SDK) {
		if smp != nil {
			s.sampler = smp
		}
	}
}

// WithErrorHandler routes SDK-internal errors (export failures,
// collector rejections) into a caller-supplied function. If unset, errors
// are sent to the standard logger.
func WithErrorHandler(fn func(err error)) SDKOption {
	return func(s *SDK) {
		if fn != nil {
			s.errorHandler = fn
		}
	}
}

// WithAutoResource enables OTel semantic-convention resource attributes
// (process.pid, runtime.name, host.arch, etc). Default off.
func WithAutoResource(enabled bool) SDKOption {
	return func(s *SDK) {
		s.autoResource = enabled
	}
}

// handleError dispatches an SDK-internal error. Safe with nil handler.
func (s *SDK) handleError(err error) {
	if s.errorHandler != nil {
		s.errorHandler(err)
		return
	}
	log.Printf("%v", err)
}

// Version is the SDK version string. Override via -ldflags at build time.
var Version = "0.1.0"

// New builds an SDK. The service resource attribute is required.
func New(endpoint string, opts ...SDKOption) (*SDK, error) {
	s := &SDK{
		exporter:      NewExporter(endpoint),
		resource:      map[string]string{},
		flushInterval: 2 * time.Second,
		maxBatch:      500,
		sampler:       AlwaysOnSampler{},
		stop:          make(chan struct{}),
		done:          make(chan struct{}),
	}
	for _, opt := range opts {
		opt(s)
	}

	sname, ok := s.resource["service.name"]
	if !ok || sname == "" {
		return nil, errors.New("otlp: WithService(name) is required")
	}

	s.resource["telemetry.sdk.name"] = "otlp-go"
	s.resource["telemetry.sdk.language"] = "go"
	s.resource["telemetry.sdk.version"] = Version

	if s.autoResource {
		applyAutoResource(s.resource)
	}

	resCopy := make(map[string]string, len(s.resource))
	for k, v := range s.resource {
		resCopy[k] = v
	}
	s.buf = buffer.New(sname, resCopy)

	s.wg.Add(1)
	go s.run()
	return s, nil
}

// Log emits a single log record.
func (s *SDK) Log(ctx context.Context, severity Severity, body string, kvs ...KV) {
	s.LogAttrs(ctx, LogRecord{
		Severity:   severity,
		Body:       body,
		Attributes: Map(kvs...),
	})
}

// LogAttrs emits a pre-built LogRecord.
func (s *SDK) LogAttrs(ctx context.Context, l LogRecord) {
	s.buf.PushLog(buffer.LogRecord{
		Timestamp:  l.Timestamp,
		Service:    l.Service,
		Severity:   transform.NormalizeSeverity(string(l.Severity)),
		Body:       l.Body,
		Attributes: l.Attributes,
		TraceID:    l.TraceID,
		SpanID:     l.SpanID,
	})
}

// Counter emits a counter metric.
func (s *SDK) Counter(ctx context.Context, name string, value float64, kvs ...KV) {
	s.emitMetric(MetricPoint{
		Name:   name,
		Value:  value,
		Type:   TypeCounter,
		Labels: Map(kvs...),
	})
}

// Gauge emits a gauge metric.
func (s *SDK) Gauge(ctx context.Context, name string, value float64, kvs ...KV) {
	s.emitMetric(MetricPoint{
		Name:   name,
		Value:  value,
		Type:   TypeGauge,
		Labels: Map(kvs...),
	})
}

// Histogram emits a single histogram sample.
func (s *SDK) Histogram(ctx context.Context, name string, value float64, kvs ...KV) {
	s.emitMetric(MetricPoint{
		Name:   name,
		Value:  value,
		Type:   TypeHistogram,
		Labels: Map(kvs...),
	})
}

func (s *SDK) emitMetric(m MetricPoint) {
	s.buf.PushMetric(buffer.MetricPoint{
		Timestamp: m.Timestamp,
		Service:   m.Service,
		Name:      m.Name,
		Value:     m.Value,
		Unit:      m.Unit,
		Type:      transform.NormalizeMetricType(string(m.Type)),
		Labels:    m.Labels,
	})
}

// Record emits a span around a function call. The returned func() should
// be deferred. If err is non-nil, the span is marked error.
//
// Example:
//
//	defer sdk.Record(ctx, "GET /checkout", time.Now(), nil)(
//	    otlp.String("user_id", "u-42"),
//	)
func (s *SDK) Record(ctx context.Context, name string, start time.Time, err error) func(kvs ...KV) {
	tid := getTraceID(ctx)
	sid := transformSpanID()
	pid := getParentSpanID(ctx)

	return func(kvs ...KV) {
		dur := time.Since(start).Milliseconds()
		status := StatusOK
		if err != nil {
			status = StatusError
		}
		s.buf.PushSpan(buffer.SpanRecord{
			TraceID:    tid,
			SpanID:     sid,
			ParentID:   pid,
			Name:       name,
			StartTime:  start,
			DurationMs: dur,
			Status:     string(status),
			Attributes: Map(kvs...),
		})
	}
}

// Trace starts a fresh trace and returns a context with the trace ID
// injected, plus a callback that emits the root span on completion.
func (s *SDK) Trace(ctx context.Context, name string) (context.Context, func(err error)) {
	tid, sid := transformTraceAndSpan()
	ctx = withTrace(ctx, tid, sid)
	start := time.Now()
	sampled := s.sampler.ShouldSample(SampleContext{
		TraceID: tid, SpanID: sid, Name: name,
		HasParent: getParentSpanID(ctx) != "",
	})
	if !sampled {
		s.stats.SamplerSkipped.Add(1)
	}
	return ctx, func(err error) {
		if !sampled {
			return // drop on the floor
		}
		status := StatusOK
		if err != nil {
			status = StatusError
		}
		s.buf.PushSpan(buffer.SpanRecord{
			TraceID:    tid,
			SpanID:     sid,
			Name:       name,
			StartTime:  start,
			DurationMs: time.Since(start).Milliseconds(),
			Status:     string(status),
		})
	}
}

// ForceFlush drains the buffer immediately and returns when done.
func (s *SDK) ForceFlush(ctx context.Context) error {
	return s.flush(ctx)
}

// Snapshot returns the current pending buffer as an otlp.Request without
// clearing it. Useful for tools like PrometheusCollector that want to
// render the current state on demand.
func (s *SDK) Snapshot() Request {
	return requestFromBuffer(s.buf.Snapshot())
}

// Shutdown flushes pending records and stops the worker.
func (s *SDK) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	if s.inShutdown {
		s.mu.Unlock()
		return nil
	}
	s.inShutdown = true
	s.mu.Unlock()

	close(s.stop)
	select {
	case <-s.done:
	case <-ctx.Done():
		return ctx.Err()
	}
	return s.flush(ctx)
}

// run is the background flusher.
func (s *SDK) run() {
	defer close(s.done)
	t := time.NewTicker(s.flushInterval)
	defer t.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-t.C:
			if err := s.flush(context.Background()); err != nil {
				log.Printf("otlp: flush failed: %v", err)
			}
		}
	}
}

func (s *SDK) flush(ctx context.Context) error {
	s.stats.FlushCalls.Add(1)
	breq := s.buf.Drain()
	if len(breq.Logs) == 0 && len(breq.Metrics) == 0 && len(breq.Spans) == 0 {
		return nil
	}
	if s.maxBatch > 0 {
		before := len(breq.Logs) + len(breq.Metrics) + len(breq.Spans)
		breq = trimBufferBatch(breq, s.maxBatch)
		after := len(breq.Logs) + len(breq.Metrics) + len(breq.Spans)
		if diff := int64(before - after); diff > 0 {
			s.stats.DroppedLogs.Add(diff)
		}
	}
	req := requestFromBuffer(breq)
	resp, err := s.exporter.Export(ctx, req)
	if err != nil {
		s.stats.FlushErrors.Add(1)
		s.handleError(fmt.Errorf("otlp: export failed (logs=%d metrics=%d spans=%d): %w",
			len(req.Logs), len(req.Metrics), len(req.Spans), err))
		s.requeue(breq)
		return err
	}
	s.stats.LogsEmitted.Add(int64(len(req.Logs)))
	s.stats.MetricsEmitted.Add(int64(len(req.Metrics)))
	s.stats.SpansEmitted.Add(int64(len(req.Spans)))
	if len(resp.Errors) > 0 {
		for _, e := range resp.Errors {
			s.handleError(fmt.Errorf("otlp: collector rejected: %s", e))
		}
	}
	return nil
}

func (s *SDK) requeue(breq buffer.Request) {
	s.stats.RequeuedLogs.Add(int64(len(breq.Logs)))
	s.stats.RequeuedMetrics.Add(int64(len(breq.Metrics)))
	s.stats.RequeuedSpans.Add(int64(len(breq.Spans)))
	for _, l := range breq.Logs {
		s.buf.PushLog(l)
	}
	for _, m := range breq.Metrics {
		s.buf.PushMetric(m)
	}
	for _, sp := range breq.Spans {
		s.buf.PushSpan(sp)
	}
}

// requestFromBuffer converts the buffer-local types to the public wire types.
func requestFromBuffer(breq buffer.Request) Request {
	req := Request{
		ResourceAttrs: breq.ResourceAttrs,
		Logs:          make([]LogRecord, len(breq.Logs)),
		Metrics:       make([]MetricPoint, len(breq.Metrics)),
		Spans:         make([]SpanRecord, len(breq.Spans)),
	}
	for i, l := range breq.Logs {
		req.Logs[i] = LogRecord{
			Timestamp:  l.Timestamp,
			Service:    l.Service,
			Severity:   Severity(l.Severity),
			Body:       l.Body,
			Attributes: l.Attributes,
			TraceID:    l.TraceID,
			SpanID:     l.SpanID,
		}
	}
	for i, m := range breq.Metrics {
		req.Metrics[i] = MetricPoint{
			Timestamp: m.Timestamp,
			Service:   m.Service,
			Name:      m.Name,
			Value:     m.Value,
			Unit:      m.Unit,
			Type:      MetricType(m.Type),
			Labels:    m.Labels,
		}
	}
	for i, sp := range breq.Spans {
		req.Spans[i] = SpanRecord{
			TraceID:    sp.TraceID,
			SpanID:     sp.SpanID,
			ParentID:   sp.ParentID,
			Name:       sp.Name,
			Service:    sp.Service,
			StartTime:  sp.StartTime,
			DurationMs: sp.DurationMs,
			Status:     SpanStatus(sp.Status),
			Attributes: sp.Attributes,
		}
	}
	return req
}

// trimBufferBatch caps the total record count on the buffer-local types.
// Logs are preferred; spans are trimmed first when over budget because
// they carry the least user-facing signal volume per record.
func trimBufferBatch(req buffer.Request, max int) buffer.Request {
	total := len(req.Logs) + len(req.Metrics) + len(req.Spans)
	if total <= max {
		return req
	}
	if l := len(req.Logs); l > max {
		req.Logs = req.Logs[l-max:]
		return req
	}
	budget := max - len(req.Logs)
	if m := len(req.Metrics); m > budget {
		removed := m - budget
		req.Metrics = req.Metrics[removed:]
		return req
	}
	budget -= len(req.Metrics)
	if sp := len(req.Spans); sp > budget {
		req.Spans = req.Spans[sp-budget:]
	}
	return req
}

// contextKey is unexported so external packages cannot collide.
type contextKey int

const (
	traceKey contextKey = iota
	spanKey
)

func withTrace(ctx context.Context, traceID, spanID string) context.Context {
	ctx = context.WithValue(ctx, traceKey, traceID)
	ctx = context.WithValue(ctx, spanKey, spanID)
	return ctx
}

func getTraceID(ctx context.Context) string {
	if ctx == nil {
		return transformTraceID()
	}
	if v, ok := ctx.Value(traceKey).(string); ok && v != "" {
		return v
	}
	return transformTraceID()
}

func getParentSpanID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(spanKey).(string); ok {
		return v
	}
	return ""
}

// Tiny aliases so the SDK code reads naturally without exposing the
// transform package to callers.
func transformTraceAndSpan() (string, string) { return transform.Hex(16), transform.Hex(8) }
func transformSpanID() string                  { return transform.Hex(8) }
func transformTraceID() string                 { return transform.Hex(16) }
