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

// TraceRoundTripper 向每个出站请求注入 W3C trace context
// replica request.
type TraceRoundTripper struct {
	Inner  http.RoundTripper
	Tracer Tracer
}

// NewTraceRT 用 trace 注入包装现有的 RoundTripper。
func NewTraceRT(inner http.RoundTripper, t Tracer) *TraceRoundTripper {
	if inner == nil {
		inner = http.DefaultTransport
	}
	return &TraceRoundTripper{Inner: inner, Tracer: t}
}

// RoundTrip 在每个请求上注入 traceparent 头。
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

// ExtractTrace 以自包含的方式解析 traceparent。
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

// TraceFromContext 从 ctx 提取 traceparent 字符串。
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

// WithTraceparent 在 context 上存储 traceparent 字符串。
func WithTraceparent(ctx context.Context, tp string) context.Context {
	return context.WithValue(ctx, traceparentKey{}, tp)
}

type traceparentKey struct{}
