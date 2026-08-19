package replica

import (
	"context"
	"net/http"
)

// Tracer is the minimal interface replica needs to forward W3C
// trace context. The api package implements it; replica depends
// only on this contract.
type Tracer interface {
	Header(ctx context.Context) (traceparent, tracestate string)
	NewSpan(ctx context.Context, name string) (newCtx context.Context, traceparent, tracestate string)
}

// TraceRoundTripper injects W3C trace context into every outbound
// replica request.
type TraceRoundTripper struct {
	Inner  http.RoundTripper
	Tracer Tracer
}

// NewTraceRT wraps an existing RoundTripper with trace injection.
func NewTraceRT(inner http.RoundTripper, t Tracer) *TraceRoundTripper {
	if inner == nil {
		inner = http.DefaultTransport
	}
	return &TraceRoundTripper{Inner: inner, Tracer: t}
}

// RoundTrip injects the traceparent header on every request.
func (rt *TraceRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if rt.Tracer != nil {
		if tp, ts := rt.Tracer.Header(req.Context()); tp != "" {
			req.Header.Set("traceparent", tp)
			if ts != "" {
				req.Header.Set("tracestate", ts)
			}
		}
	}
	return rt.Inner.RoundTrip(req)
}

// ExtractTrace parses traceparent in a self-contained way.
func ExtractTrace(r *http.Request) (traceID, spanID, flags string) {
	tp := r.Header.Get("traceparent")
	if tp == "" || len(tp) < 55 {
		return "", "", ""
	}
	dashes := []int{}
	for i := 0; i < len(tp); i++ {
		if tp[i] == '-' {
			dashes = append(dashes, i)
			if len(dashes) == 3 {
				break
			}
		}
	}
	if len(dashes) < 3 {
		return "", "", ""
	}
	return tp[dashes[0]+1 : dashes[1]], tp[dashes[1]+1 : dashes[2]], tp[dashes[2]+1 :]
}

// TraceFromContext extracts a traceparent string from ctx.
func TraceFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v := ctx.Value(traceparentKey{}); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// WithTraceparent stores a traceparent string on the context.
func WithTraceparent(ctx context.Context, tp string) context.Context {
	return context.WithValue(ctx, traceparentKey{}, tp)
}

type traceparentKey struct{}
