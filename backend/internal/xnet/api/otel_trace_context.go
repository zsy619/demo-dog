package api

import (
	"net/http"
	"regexp"
	"strings"
)

// W3C Trace Context (https://www.w3.org/TR/trace-context/) parser.
//
// A `traceparent` header has the form:
//
//   version-traceid-spanid-flags
//
// where version is two hex chars, traceid is 32 hex chars, spanid is
// 16 hex chars, flags is two hex chars (bit 0 = sampled).
//
// Example:
//
//   traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01
//
// `tracestate` is a comma-separated list of vendor-specific
// key=value pairs that augment the trace. We accept and re-emit it
// verbatim.

type TraceContext struct {
	Version    string
	TraceID    string
	SpanID     string
	Flags      string
	Tracestate string
	Sampled    bool
}

var (
	tpidRE      = regexp.MustCompile(`^([0-9a-fA-F]{2})-([0-9a-fA-F]{32})-([0-9a-fA-F]{16})-([0-9a-fA-F]{2})$`)
	invalidHex  = regexp.MustCompile(`^0+$`)
)

// ParseTraceContext extracts the W3C trace context from request
// headers. Returns nil if absent or invalid; callers can fall back to
// generating a fresh context.
func ParseTraceContext(r *http.Request) *TraceContext {
	tp := strings.TrimSpace(r.Header.Get("traceparent"))
	if tp == "" {
		return nil
	}
	m := tpidRE.FindStringSubmatch(tp)
	if m == nil {
		return nil
	}
	traceID := strings.ToLower(m[2])
	spanID := strings.ToLower(m[3])
	// All-zero trace_id or span_id is invalid per spec.
	if invalidHex.MatchString(traceID) || invalidHex.MatchString(spanID) {
		return nil
	}
	flags := m[4]
	sampled := false
	// Bit 0 of the first nibble is the sampled flag.
	if len(flags) >= 1 {
		// The flags byte is encoded as two hex chars: flags[0] is the
		// high nibble and flags[1] is the low nibble. The sampled flag
		// is bit 0 of the byte.
		hi := hexNibble(flags[0])
		lo := hexNibble(flags[1])
		if hi >= 0 && lo >= 0 && ((hi<<4 | lo) & 1) == 1 {
			sampled = true
		}
	}
	return &TraceContext{
		Version:    m[1],
		TraceID:    traceID,
		SpanID:     spanID,
		Flags:      flags,
		Tracestate: r.Header.Get("tracestate"),
		Sampled:    sampled,
	}
}

// InjectTraceContext writes the W3C trace context into the response
// headers so callers downstream can stitch their spans to the same
// trace.
func InjectTraceContext(rw http.ResponseWriter, tc *TraceContext) {
	if tc == nil {
		return
	}
	traceparent := tc.Version + "-" + tc.TraceID + "-" + tc.SpanID + "-" + tc.Flags
	rw.Header().Set("traceparent", traceparent)
	if tc.Tracestate != "" {
		rw.Header().Set("tracestate", tc.Tracestate)
	}
}

// GenerateTraceContext returns a fresh trace + span id pair. Used
// when a request has no incoming context and we still want to
// attach one to the response.
func GenerateTraceContext() *TraceContext {
	return &TraceContext{
		Version: "00",
		TraceID: randomHex(32),
		SpanID:  randomHex(16),
		Flags:   "01",
		Sampled: true,
	}
}

// childSpanID generates a fresh 16-hex span id; the helper exists so
// trace propagation code is read top-down.
func childSpanID() string { return randomHex(16) }

// randomHex produces n hex characters from crypto/rand via stdlib.
// We import here so trace context generation stays in this file.
func randomHex(n int) string {
	const hex = "0123456789abcdef"
	b := make([]byte, n)
	// Use crypto/rand via the stdlib. Inline-importing avoids creating
	// an import cycle with the api package.
	r := cryptoRandBytes((n + 1) / 2)
	for i := range b {
		if i%2 == 0 {
			b[i] = hex[r[i/2]>>4]
		} else {
			b[i] = hex[r[i/2]&0x0f]
		}
	}
	return string(b)
}

// hexNibble returns 0..15 for a hex character, or -1 if not a hex char.
func hexNibble(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}
