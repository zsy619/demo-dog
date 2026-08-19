// W3C Trace Context propagation.
//
// The SDK follows the W3C Trace Context spec for cross-service trace
// propagation. Outbound HTTP requests should carry a `traceparent`
// header; inbound requests should be parsed and used to continue the
// caller's trace.
//
// Header format (one line, space-separated):
//
//   traceparent: <version>-<trace_id>-<span_id>-<flags>
//
// Where:
//   - version: 2 hex chars (current spec uses "00")
//   - trace_id: 16 bytes = 32 hex chars
//   - span_id: 8 bytes = 16 hex chars
//   - flags: 2 hex chars (01 = sampled)
//
// The companion `tracestate` header is forwarded verbatim when present.
package otlp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
)

// Propagator injects and extracts trace context into carrier objects.
// We support the W3C `traceparent` and `tracestate` headers and a
// stdlib *http.Request adapter.
type Propagator struct{}

// NewPropagator returns the default W3C propagator.
func NewPropagator() *Propagator { return &Propagator{} }

// traceparentRegex matches the version-traceid-spanid-flags shape.
var traceparentRegex = regexp.MustCompile(`^[0-9a-fA-F]{2}-([0-9a-fA-F]{32})-([0-9a-fA-F]{16})-[0-9a-fA-F]{2}$`)

// TraceContext is the parsed trace context we carry across services.
type TraceContext struct {
	TraceID    string
	SpanID     string
	ParentID   string
	Flags      string
	Tracestate string
	Sampled    bool
}

// Inject writes a W3C traceparent header onto the supplied header map.
// If no trace context is present in ctx (i.e. caller did not call
// sdk.Trace or sdk.Record), this is a no-op.
func (p *Propagator) Inject(ctx context.Context, headers http.Header) {
	tid := getTraceID(ctx)
	sid := getParentSpanID(ctx)
	if tid == "" || sid == "" {
		return
	}
	flags := "00"
	if sampled, ok := ctx.Value(sampledKey).(bool); ok && sampled {
		flags = "01"
	}
	headers.Set("traceparent", fmt.Sprintf("00-%s-%s-%s", tid, sid, flags))
	if ts, ok := ctx.Value(tracestateKey).(string); ok && ts != "" {
		headers.Set("tracestate", ts)
	}
}

// InjectHTTPHeader is a convenience wrapper for *http.Request.
func (p *Propagator) InjectHTTPHeader(ctx context.Context, req *http.Request) {
	p.Inject(ctx, req.Header)
}

// Extract reads a W3C traceparent header from the supplied headers.
// Returns nil if the header is missing or malformed.
func (p *Propagator) Extract(headers http.Header) *TraceContext {
	tp := headers.Get("traceparent")
	if tp == "" {
		return nil
	}
	m := traceparentRegex.FindStringSubmatch(tp)
	if m == nil {
		return nil
	}
	flags := tp[len(tp)-2:]
	return &TraceContext{
		TraceID:    m[1],
		SpanID:     m[2],
		ParentID:   m[2], // the incoming span becomes our parent
		Flags:      flags,
		Sampled:    flags == "01",
		Tracestate: headers.Get("tracestate"),
	}
}

// ExtractHTTPHeader is a convenience wrapper for *http.Request.
func (p *Propagator) ExtractHTTPHeader(req *http.Request) *TraceContext {
	return p.Extract(req.Header)
}

// WithTraceContext returns a context carrying the supplied TraceContext.
// Subsequent sdk.Trace / sdk.Record calls will pick up the trace_id and
// parent_id from this context.
func (p *Propagator) WithTraceContext(ctx context.Context, tc *TraceContext) context.Context {
	if tc == nil {
		return ctx
	}
	ctx = withTrace(ctx, tc.TraceID, tc.SpanID)
	ctx = context.WithValue(ctx, sampledKey, tc.Sampled)
	if tc.Tracestate != "" {
		ctx = context.WithValue(ctx, tracestateKey, tc.Tracestate)
	}
	return ctx
}

// TraceparentString returns the canonical traceparent header value.
func (tc *TraceContext) TraceparentString() string {
	flags := tc.Flags
	if flags == "" {
		if tc.Sampled {
			flags = "01"
		} else {
			flags = "00"
		}
	}
	return fmt.Sprintf("00-%s-%s-%s", tc.TraceID, tc.SpanID, flags)
}

// Validate checks the trace_id / span_id are well-formed.
func (tc *TraceContext) Validate() error {
	if len(tc.TraceID) != 32 {
		return errors.New("trace_id must be 32 hex chars (16 bytes)")
	}
	if len(tc.SpanID) != 16 {
		return errors.New("span_id must be 16 hex chars (8 bytes)")
	}
	return nil
}

// Internal context keys used for sampled + tracestate.
const (
	sampledKey    contextKey = 100
	tracestateKey contextKey = 101
)

// WithSampled returns a context carrying a sampled flag. The Propagator
// uses the flag to pick the traceparent flags byte ("01" when true).
func WithSampled(ctx context.Context, sampled bool) context.Context {
	return context.WithValue(ctx, sampledKey, sampled)
}

// WithTraceID returns a context carrying the given 32-hex trace id.
// Used by callers that want to start a child of an inbound trace.
func WithTraceID(ctx context.Context, id string) context.Context {
	ctx = context.WithValue(ctx, traceKey, id)
	return ctx
}

// WithParentSpanID returns a context carrying the 16-hex span id of
// the caller's parent span. The Propagator emits this as the parent-id
// component of the next traceparent.
func WithParentSpanID(ctx context.Context, id string) context.Context {
	ctx = context.WithValue(ctx, spanKey, id)
	return ctx
}

// WithTracestate attaches a vendor-specific tracestate header value.
func WithTracestate(ctx context.Context, state string) context.Context {
	return context.WithValue(ctx, tracestateKey, state)
}

// Sampled reports whether ctx carries a true Sampled flag.
func Sampled(ctx context.Context) bool {
	if v, ok := ctx.Value(sampledKey).(bool); ok {
		return v
	}
	return false
}
